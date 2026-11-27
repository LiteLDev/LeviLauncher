/* SPDX-License-Identifier: GPL-3.0-only
 *
 * libLeviMetalFX.dylib:
 *   1. Configures Wine's CAMetalLayer as opaque with non-transaction presentation
 *      to reduce compositing jitter and help macOS select direct presentation.
 *   2. When LEVI_METALFX_SPATIAL_FACTOR > 1.0 (paired with virtual window scaling
 *      on the Windows side), hands D3DMetal a lower-resolution private MTLTexture
 *      proxy drawable and upscales zero-copy to the native CAMetalDrawable via
 *      Apple's native MTLFXSpatialScaler inside the game's MTLCommandBuffer.
 *   3. When LEVI_METALFX_SHARPNESS > 0.0, runs a Contrast-Adaptive Sharpening
 *      (CAS) Metal render pass into the native CAMetalDrawable — either after
 *      MTLFXSpatialScaler (D3DMetal) or directly on the native-sized proxy
 *      frame in LEVI_METALFX_SHARPEN_ONLY=1 mode (Mullion / VKD3D-Proton).
 */
#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#import <QuartzCore/QuartzCore.h>
#import <Metal/Metal.h>
#import <MetalFX/MetalFX.h>
#import <objc/runtime.h>
#import <errno.h>
#import <fcntl.h>
#import <math.h>
#import <sched.h>
#import <spawn.h>
#import <stdio.h>
#import <stdlib.h>
#import <string.h>
#import <time.h>
#import <unistd.h>
#import <sys/wait.h>
#import "function_keys.h"

static const void *kLeviSpatialStateKey = &kLeviSpatialStateKey;
static const void *kLeviPendingPresentsKey = &kLeviPendingPresentsKey;
static double g_spatial_factor = 1.0;
static BOOL g_retina_upscale = NO;
static float g_sharpness = 0.0f;  /* 0.0 = off, (0.0..1.0] = CAS sharpening strength */
static BOOL g_sharpen_only = NO;  /* YES = sharpen native-size frames without spatial scaling */
static BOOL g_opaque_policy = YES;
static BOOL g_debug_log = NO;
static int g_vsync_override = -1; /* -1 = game default, 0 = force off, 1 = force on */
static int g_max_fps = 0;         /* 0 = unlimited, 1..1000 = native Metal frame pacing cap */
static NSUInteger g_drawable_count = 2; /* 2 = low latency & avoids macOS windowed 60fps stall; override via LEVI_METAL_DRAWABLE_COUNT */
static uint64_t g_frame_interval_ns = 0;
static uint64_t g_drawable_seq = 0;
static CGFloat g_main_display_scale = 1.0;

static NSString *g_game_icon_path = nil;
static NSString *g_game_title = nil;
static NSImage *g_cached_dock_icon = nil;
static dispatch_once_t g_dock_icon_once;

static IMP g_orig_setDrawableSize = NULL;
static IMP g_orig_setContentsScale = NULL;
static IMP g_orig_setOpaque = NULL;
static IMP g_orig_setPresentsWithTransaction = NULL;
static IMP g_orig_setDisplaySyncEnabled = NULL;
static IMP g_orig_setMaximumDrawableCount = NULL;
static IMP g_orig_setFramebufferOnly = NULL;
static IMP g_orig_nextDrawable = NULL;
static IMP g_orig_setAppIcon = NULL;
static IMP g_orig_setWindowTitle = NULL;
static IMP g_orig_setMainMenu = NULL;

typedef void (*SetDrawableSizeFn)(id, SEL, CGSize);
typedef void (*SetCGFloatFn)(id, SEL, CGFloat);
typedef void (*SetBoolFn)(id, SEL, BOOL);
typedef void (*SetNSUIntegerFn)(id, SEL, NSUInteger);
typedef id<CAMetalDrawable> (*NextDrawableFn)(id, SEL);
typedef void (*PresentDrawableFn)(id, SEL, id<MTLDrawable>);
typedef void (*PresentDrawableAtTimeFn)(id, SEL, id<MTLDrawable>, CFTimeInterval);
typedef void (*PresentDrawableAfterMinFn)(id, SEL, id<MTLDrawable>, CFTimeInterval);
typedef void (*DrawablePresentTimeFn)(id, SEL, CFTimeInterval);
typedef void (*CommitFn)(id, SEL);
typedef void (*SetAppIconFn)(id, SEL, NSImage *);
typedef void (*SetWindowTitleFn)(id, SEL, NSString *);
typedef void (*SetMainMenuFn)(id, SEL, NSMenu *);

static NSMutableSet<NSString *> *g_swizzled_cmdbuf_methods = nil;
static NSMutableSet<NSString *> *g_swizzled_cmdbuf_classes = nil;
static NSMutableSet<NSString *> *g_swizzled_drawable_methods = nil;
static NSMutableSet<NSString *> *g_swizzled_drawable_classes = nil;
static dispatch_once_t g_tables_once;
static int g_logged_present_bypass = 0;

static void levi_ensure_tables(void) {
    dispatch_once(&g_tables_once, ^{
        g_swizzled_cmdbuf_methods = [NSMutableSet new];
        g_swizzled_cmdbuf_classes = [NSMutableSet new];
        g_swizzled_drawable_methods = [NSMutableSet new];
        g_swizzled_drawable_classes = [NSMutableSet new];
    });
}

static BOOL levi_is_wine_metal_layer(CAMetalLayer *layer) {
    if (!layer) return NO;
    /* Use -class instead of object_getClass so KVO-notifying subclasses match. */
    NSString *layerClass = NSStringFromClass([layer class]);
    if ([layerClass containsString:@"WineMetalLayer"] ||
        [layerClass containsString:@"LeviProbeMetalLayer"]) {
        return YES;
    }
    id delegate = layer.delegate;
    if (delegate) {
        NSString *delClass = NSStringFromClass([delegate class]);
        if ([delClass containsString:@"WineMetalView"] ||
            [delClass containsString:@"WineContentView"] ||
            [delClass containsString:@"MullionMetalView"] ||
            [delClass containsString:@"LeviProbeMetalView"]) {
            return YES;
        }
    }
    return NO;
}

static CGFloat levi_query_main_display_scale(void) {
    CGDirectDisplayID mainDisplay = CGMainDisplayID();
    CGDisplayModeRef mode = CGDisplayCopyDisplayMode(mainDisplay);
    CGFloat scale = 1.0;
    if (mode) {
        size_t pw = CGDisplayModeGetPixelWidth(mode);
        size_t w = CGDisplayModeGetWidth(mode);
        if (w > 0 && pw > w) {
            scale = (CGFloat)pw / (CGFloat)w;
        }
        CGDisplayModeRelease(mode);
    }
    return scale >= 1.0 ? scale : 1.0;
}

static CGFloat levi_desired_backing_scale(CAMetalLayer *layer) {
    CGFloat cur = layer.contentsScale > 0.0 ? layer.contentsScale : 1.0;
    if (!g_retina_upscale) return cur;
    CGFloat target = g_main_display_scale;
    if ([NSThread isMainThread]) {
        id delegate = layer.delegate;
        if (delegate && [delegate isKindOfClass:[NSView class]]) {
            NSWindow *win = ((NSView *)delegate).window;
            if (win && win.backingScaleFactor > 0.5) {
                target = win.backingScaleFactor;
            }
        }
    }
    return target > cur ? target : cur;
}

static BOOL levi_spatial_enabled(void) {
    return g_spatial_factor > 1.01 || g_retina_upscale;
}

static BOOL levi_sharpen_enabled(void) {
    return g_sharpness > 0.001f;
}

static BOOL levi_sharpen_only_enabled(void) {
    return g_sharpen_only && levi_sharpen_enabled() && !levi_spatial_enabled();
}

static BOOL levi_is_hdr_format(MTLPixelFormat fmt) {
    switch (fmt) {
    case MTLPixelFormatRGBA16Float:
    case MTLPixelFormatRGBA32Float:
        return YES;
    default:
        return NO;
    }
}

typedef struct {
    float sharpness;
    uint32_t isHDR;
} LeviCASParams;

static NSString *const kLeviCASShaderSource =
    @"#include <metal_stdlib>\n"
     "using namespace metal;\n"
     "struct LeviCASParams {\n"
     "    float sharpness;\n"
     "    uint isHDR;\n"
     "};\n"
     "struct LeviCASOut {\n"
     "    float4 position [[position]];\n"
     "    float2 uv;\n"
     "};\n"
     "vertex LeviCASOut levi_cas_vertex(uint vid [[vertex_id]]) {\n"
     "    LeviCASOut out;\n"
     "    float2 pos = float2((vid == 1) ? 3.0f : -1.0f, (vid == 2) ? 3.0f : -1.0f);\n"
     "    out.position = float4(pos, 0.0f, 1.0f);\n"
     "    out.uv = float2(pos.x * 0.5f + 0.5f, 0.5f - pos.y * 0.5f);\n"
     "    return out;\n"
     "}\n"
     "fragment float4 levi_cas_fragment(LeviCASOut in [[stage_in]],\n"
     "                                  texture2d<float> src [[texture(0)]],\n"
     "                                  constant LeviCASParams &params [[buffer(0)]]) {\n"
     "    constexpr sampler s(coord::normalized, filter::linear, address::clamp_to_edge);\n"
     "    uint texW = src.get_width();\n"
     "    uint texH = src.get_height();\n"
     "    float2 texel = float2(1.0f / float(max(texW, 1u)), 1.0f / float(max(texH, 1u)));\n"
     "    float4 center = src.sample(s, in.uv);\n"
     "    float3 a = src.sample(s, in.uv + float2(-texel.x, -texel.y)).rgb;\n"
     "    float3 b = src.sample(s, in.uv + float2(    0.0f, -texel.y)).rgb;\n"
     "    float3 c = src.sample(s, in.uv + float2( texel.x, -texel.y)).rgb;\n"
     "    float3 d = src.sample(s, in.uv + float2(-texel.x,     0.0f)).rgb;\n"
     "    float3 e = center.rgb;\n"
     "    float3 f = src.sample(s, in.uv + float2( texel.x,     0.0f)).rgb;\n"
     "    float3 g = src.sample(s, in.uv + float2(-texel.x,  texel.y)).rgb;\n"
     "    float3 h = src.sample(s, in.uv + float2(    0.0f,  texel.y)).rgb;\n"
     "    float3 i = src.sample(s, in.uv + float2( texel.x,  texel.y)).rgb;\n"
     "    float3 mn = min(min(min(d, e), min(f, b)), h);\n"
     "    float3 mn2 = min(mn, min(min(a, c), min(g, i)));\n"
     "    mn += mn2;\n"
     "    float3 mx = max(max(max(d, e), max(f, b)), h);\n"
     "    float3 mx2 = max(mx, max(max(a, c), max(g, i)));\n"
     "    mx += mx2;\n"
     "    float3 rcpMx = 1.0f / max(mx, float3(1e-5f));\n"
     "    float3 amp = saturate(min(mn, max(2.0f - mx, float3(0.0f))) * rcpMx);\n"
     "    amp = sqrt(amp);\n"
     "    float peak = -1.0f / mix(8.0f, 5.0f, saturate(params.sharpness));\n"
     "    float3 wRGB = amp * peak;\n"
     "    float3 outRGB = (wRGB * (b + d + f + h) + e) / (1.0f + 4.0f * wRGB);\n"
     "    if (params.isHDR != 0u) {\n"
     "        outRGB = max(outRGB, float3(0.0f));\n"
     "    } else {\n"
     "        outRGB = saturate(outRGB);\n"
     "    }\n"
     "    return float4(outRGB, center.a);\n"
     "}\n";

static CGSize levi_native_output_size(CAMetalLayer *layer, CGSize fallback) {
    CGRect bounds = layer.bounds;
    CGFloat scale = levi_desired_backing_scale(layer);
    CGFloat w = floor(bounds.size.width * scale + 0.5);
    CGFloat h = floor(bounds.size.height * scale + 0.5);
    if (w >= 64.0 && h >= 64.0) {
        return CGSizeMake(w, h);
    }
    if (fallback.width >= 64.0 && fallback.height >= 64.0) {
        return fallback;
    }
    CGSize current = layer.drawableSize;
    if (current.width >= 64.0 && current.height >= 64.0) {
        return current;
    }
    return CGSizeZero;
}

static CGSize levi_scaled_input_size(CGSize outputSize, double factor) {
    if (factor <= 1.01 || outputSize.width < 64.0 || outputSize.height < 64.0) {
        return outputSize;
    }
    CGFloat w = floor(outputSize.width / factor + 0.5);
    CGFloat h = floor(outputSize.height / factor + 0.5);
    if (w < 64.0) w = 64.0;
    if (h < 64.0) h = 64.0;
    if (w >= outputSize.width) w = outputSize.width - 1.0;
    if (h >= outputSize.height) h = outputSize.height - 1.0;
    return CGSizeMake(w, h);
}

static MTLFXSpatialScalerColorProcessingMode levi_color_mode_for_format(MTLPixelFormat fmt) {
    switch (fmt) {
    case MTLPixelFormatRGBA16Float:
    case MTLPixelFormatRGBA32Float:
    case MTLPixelFormatRGB10A2Unorm:
    case MTLPixelFormatBGR10A2Unorm:
        return MTLFXSpatialScalerColorProcessingModeHDR;
    default:
        return MTLFXSpatialScalerColorProcessingModePerceptual;
    }
}

@interface LeviSpatialLayerState : NSObject
@property (nonatomic) CGSize requestedInputSize;
@property (nonatomic) CGSize lastOutputSize;
@property (nonatomic, strong) NSMutableArray<id<MTLTexture>> *inputPool;
@property (nonatomic) NSUInteger poolIndex;
@property (nonatomic) NSUInteger poolWidth;
@property (nonatomic) NSUInteger poolHeight;
@property (nonatomic) MTLPixelFormat poolFormat;
@property (nonatomic, strong) id<MTLDevice> poolDevice;

@property (nonatomic, strong) id<MTLFXSpatialScaler> scaler;
@property (nonatomic) NSUInteger scalerInW;
@property (nonatomic) NSUInteger scalerInH;
@property (nonatomic) NSUInteger scalerOutW;
@property (nonatomic) NSUInteger scalerOutH;
@property (nonatomic) MTLPixelFormat scalerInFmt;
@property (nonatomic) MTLPixelFormat scalerOutFmt;
@property (nonatomic, strong) id<MTLDevice> scalerDevice;

@property (nonatomic, strong) id<MTLTexture> privateOutput;
@property (nonatomic) NSUInteger privateOutW;
@property (nonatomic) NSUInteger privateOutH;
@property (nonatomic) MTLPixelFormat privateOutFmt;
@property (nonatomic) MTLTextureUsage privateOutUsage;

@property (nonatomic, strong) id<MTLRenderPipelineState> casPipeline;
@property (nonatomic, strong) id<MTLDevice> casDevice;
@property (nonatomic) MTLPixelFormat casFormat;
@property (nonatomic) BOOL casFailed;

@property (nonatomic, strong) id<MTLCommandQueue> fallbackQueue;
@property (nonatomic) uint64_t nextFrameDeadlineNs;
@property (nonatomic) BOOL loggedSurface;
@property (nonatomic) BOOL loggedSpatial;
@property (nonatomic) BOOL loggedSharpenOnly;
@property (nonatomic) NSUInteger loggedInW;
@property (nonatomic) NSUInteger loggedInH;
@property (nonatomic) NSUInteger loggedOutW;
@property (nonatomic) NSUInteger loggedOutH;
@end

@implementation LeviSpatialLayerState
- (instancetype)init {
    self = [super init];
    if (self) {
        _inputPool = [NSMutableArray arrayWithCapacity:3];
    }
    return self;
}
@end

static LeviSpatialLayerState *levi_state_for_layer(CAMetalLayer *layer) {
    @synchronized(layer) {
        LeviSpatialLayerState *state = objc_getAssociatedObject(layer, kLeviSpatialStateKey);
        if (!state) {
            state = [LeviSpatialLayerState new];
            objc_setAssociatedObject(layer, kLeviSpatialStateKey, state, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
        }
        return state;
    }
}

@interface LeviScaledDrawable : NSObject <CAMetalDrawable>
@property (nonatomic, strong) id<MTLTexture> leviTexture;
@property (nonatomic, weak) CAMetalLayer *leviLayer;
@property (nonatomic, strong) LeviSpatialLayerState *state;
@property (nonatomic) NSUInteger drawableIDValue;
@property (nonatomic) CFTimeInterval presentedTimeValue;
@property (nonatomic, strong) NSMutableArray<MTLDrawablePresentedHandler> *handlers;
@property (nonatomic) BOOL presented;
@end

static id<CAMetalDrawable> levi_encode_and_acquire_real(LeviScaledDrawable *scaled, id<MTLCommandBuffer> cmdBuf);

@implementation LeviScaledDrawable
- (instancetype)initWithTexture:(id<MTLTexture>)tex
                          layer:(CAMetalLayer *)layer
                          state:(LeviSpatialLayerState *)state
                     drawableID:(NSUInteger)drawableID {
    self = [super init];
    if (self) {
        _leviTexture = tex;
        _leviLayer = layer;
        _state = state;
        _drawableIDValue = drawableID;
        _handlers = [NSMutableArray new];
    }
    return self;
}

- (id<MTLTexture>)texture {
    return _leviTexture;
}

- (CAMetalLayer *)layer {
    return _leviLayer;
}

- (NSUInteger)drawableID {
    return _drawableIDValue;
}

- (CFTimeInterval)presentedTime {
    return _presentedTimeValue;
}

- (void)addPresentedHandler:(MTLDrawablePresentedHandler)block {
    if (!block) return;
    @synchronized(self) {
        [_handlers addObject:[block copy]];
    }
}

- (void)levi_presentDirectWithMode:(int)mode timeValue:(CFTimeInterval)timeValue {
    CAMetalLayer *layer = _leviLayer;
    if (!layer || !layer.device) return;
    id<MTLCommandQueue> queue = nil;
    @synchronized(_state) {
        if (!_state.fallbackQueue) {
            _state.fallbackQueue = [layer.device newCommandQueue];
        }
        queue = _state.fallbackQueue;
    }
    if (!queue) return;
    id<MTLCommandBuffer> cmdBuf = [queue commandBuffer];
    if (!cmdBuf) return;
    id<CAMetalDrawable> real = levi_encode_and_acquire_real(self, cmdBuf);
    if (real) {
        if (mode == 1) {
            [cmdBuf presentDrawable:real atTime:timeValue];
        } else if (mode == 2) {
            [cmdBuf presentDrawable:real afterMinimumDuration:timeValue];
        } else {
            [cmdBuf presentDrawable:real];
        }
    }
    [cmdBuf commit];
}

- (void)present {
    [self levi_presentDirectWithMode:0 timeValue:0.0];
}

- (void)presentAtTime:(CFTimeInterval)presentationTime {
    if (g_vsync_override == 0) {
        [self levi_presentDirectWithMode:0 timeValue:0.0];
        return;
    }
    [self levi_presentDirectWithMode:1 timeValue:presentationTime];
}

- (void)presentAfterMinimumDuration:(CFTimeInterval)duration {
    if (g_vsync_override == 0) {
        [self levi_presentDirectWithMode:0 timeValue:0.0];
        return;
    }
    [self levi_presentDirectWithMode:2 timeValue:duration];
}
@end

static id<MTLRenderPipelineState> levi_cas_pipeline_for_state(LeviSpatialLayerState *state,
                                                             id<MTLDevice> device,
                                                             MTLPixelFormat outFmt) {
    if (!state || !device || outFmt == MTLPixelFormatInvalid) return nil;
    @synchronized(state) {
        if (state.casDevice == device && state.casFormat == outFmt) {
            if (state.casPipeline) return state.casPipeline;
            if (state.casFailed) return nil;
        }
        state.casDevice = device;
        state.casFormat = outFmt;
        state.casPipeline = nil;
        state.casFailed = NO;

        NSError *err = nil;
        id<MTLLibrary> lib = [device newLibraryWithSource:kLeviCASShaderSource options:nil error:&err];
        if (!lib) {
            state.casFailed = YES;
            fprintf(stderr, "LeviMetalFX: failed to compile CAS shader: %s\n",
                    err.localizedDescription.UTF8String ?: "unknown");
            fflush(stderr);
            return nil;
        }
        id<MTLFunction> vfn = [lib newFunctionWithName:@"levi_cas_vertex"];
        id<MTLFunction> ffn = [lib newFunctionWithName:@"levi_cas_fragment"];
        if (!vfn || !ffn) {
            state.casFailed = YES;
            return nil;
        }
        MTLRenderPipelineDescriptor *desc = [MTLRenderPipelineDescriptor new];
        desc.vertexFunction = vfn;
        desc.fragmentFunction = ffn;
        desc.colorAttachments[0].pixelFormat = outFmt;
        id<MTLRenderPipelineState> pso = [device newRenderPipelineStateWithDescriptor:desc error:&err];
        if (!pso) {
            state.casFailed = YES;
            fprintf(stderr, "LeviMetalFX: failed to create CAS pipeline (format=%lu): %s\n",
                    (unsigned long)outFmt, err.localizedDescription.UTF8String ?: "unknown");
            fflush(stderr);
            return nil;
        }
        state.casPipeline = pso;
        return pso;
    }
}

static BOOL levi_encode_cas(id<MTLCommandBuffer> cmdBuf,
                            id<MTLRenderPipelineState> pso,
                            id<MTLTexture> srcTex,
                            id<MTLTexture> dstTex,
                            float sharpness) {
    if (!cmdBuf || !pso || !srcTex || !dstTex) return NO;
    MTLRenderPassDescriptor *rpd = [MTLRenderPassDescriptor renderPassDescriptor];
    rpd.colorAttachments[0].texture = dstTex;
    rpd.colorAttachments[0].loadAction = MTLLoadActionDontCare;
    rpd.colorAttachments[0].storeAction = MTLStoreActionStore;
    id<MTLRenderCommandEncoder> enc = [cmdBuf renderCommandEncoderWithDescriptor:rpd];
    if (!enc) return NO;
    MTLViewport vp = { 0.0, 0.0, (double)dstTex.width, (double)dstTex.height, 0.0, 1.0 };
    [enc setViewport:vp];
    [enc setRenderPipelineState:pso];
    [enc setFragmentTexture:srcTex atIndex:0];
    LeviCASParams params;
    params.sharpness = sharpness;
    params.isHDR = levi_is_hdr_format(dstTex.pixelFormat) ? 1u : 0u;
    [enc setFragmentBytes:&params length:sizeof(params) atIndex:0];
    [enc drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:3];
    [enc endEncoding];
    return YES;
}

static id<CAMetalDrawable> levi_encode_and_acquire_real(LeviScaledDrawable *scaled, id<MTLCommandBuffer> cmdBuf) {
    @synchronized(scaled) {
        if (scaled.presented) return nil;
        scaled.presented = YES;
    }
    CAMetalLayer *layer = scaled.leviLayer;
    LeviSpatialLayerState *state = scaled.state;
    id<MTLTexture> inputTex = scaled.leviTexture;
    if (!layer || !state || !inputTex || !cmdBuf || !g_orig_nextDrawable) return nil;

    id<CAMetalDrawable> realDrawable = ((NextDrawableFn)g_orig_nextDrawable)(layer, @selector(nextDrawable));
    if (!realDrawable || !realDrawable.texture) return nil;

    id<MTLTexture> realTex = realDrawable.texture;
    id<MTLDevice> device = cmdBuf.device ?: layer.device;
    NSUInteger inW = inputTex.width;
    NSUInteger inH = inputTex.height;
    NSUInteger outW = realTex.width;
    NSUInteger outH = realTex.height;
    MTLPixelFormat inFmt = inputTex.pixelFormat;
    MTLPixelFormat outFmt = realTex.pixelFormat;

    BOOL scaledEncoded = NO;
    BOOL sharpenedEncoded = NO;
    BOOL directOutput = NO;

    id<MTLRenderPipelineState> casPipeline = nil;
    if (device && levi_sharpen_enabled()) {
        casPipeline = levi_cas_pipeline_for_state(state, device, outFmt);
    }

    if (device && inW >= 64 && inH >= 64 && outW > inW && outH > inH &&
        [MTLFXSpatialScalerDescriptor supportsDevice:device]) {
        id<MTLFXSpatialScaler> scaler = nil;
        id<MTLTexture> privateOut = nil;

        @synchronized(state) {
            if (!state.scaler ||
                state.scalerDevice != device ||
                state.scalerInW != inW || state.scalerInH != inH ||
                state.scalerOutW != outW || state.scalerOutH != outH ||
                state.scalerInFmt != inFmt || state.scalerOutFmt != outFmt) {
                MTLFXSpatialScalerDescriptor *desc = [MTLFXSpatialScalerDescriptor new];
                desc.inputWidth = inW;
                desc.inputHeight = inH;
                desc.outputWidth = outW;
                desc.outputHeight = outH;
                desc.colorTextureFormat = inFmt;
                desc.outputTextureFormat = outFmt;
                desc.colorProcessingMode = levi_color_mode_for_format(inFmt);
                state.scaler = [desc newSpatialScalerWithDevice:device];
                state.scalerDevice = device;
                state.scalerInW = inW;
                state.scalerInH = inH;
                state.scalerOutW = outW;
                state.scalerOutH = outH;
                state.scalerInFmt = inFmt;
                state.scalerOutFmt = outFmt;
            }
            scaler = state.scaler;

            if (scaler) {
                MTLTextureUsage reqOutUsage = scaler.outputTextureUsage;
                directOutput = (casPipeline == nil) &&
                               (realTex.storageMode == MTLStorageModePrivate) &&
                               ((realTex.usage & reqOutUsage) == reqOutUsage);
                if (!directOutput) {
                    MTLTextureUsage needUsage = reqOutUsage | MTLTextureUsageShaderRead;
                    if (!state.privateOutput ||
                        state.privateOutW != outW || state.privateOutH != outH ||
                        state.privateOutFmt != outFmt || state.privateOutUsage != needUsage) {
                        MTLTextureDescriptor *td = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:outFmt
                                                                                                        width:outW
                                                                                                       height:outH
                                                                                                    mipmapped:NO];
                        td.storageMode = MTLStorageModePrivate;
                        td.usage = needUsage;
                        state.privateOutput = [device newTextureWithDescriptor:td];
                        state.privateOutW = outW;
                        state.privateOutH = outH;
                        state.privateOutFmt = outFmt;
                        state.privateOutUsage = needUsage;
                    }
                    privateOut = state.privateOutput;
                }
            }
        }

        if (scaler && (directOutput || privateOut != nil)) {
            scaler.colorTexture = inputTex;
            scaler.inputContentWidth = inW;
            scaler.inputContentHeight = inH;
            scaler.outputTexture = directOutput ? realTex : privateOut;
            [scaler encodeToCommandBuffer:cmdBuf];
            if (!directOutput && privateOut) {
                if (casPipeline && levi_encode_cas(cmdBuf, casPipeline, privateOut, realTex, g_sharpness)) {
                    sharpenedEncoded = YES;
                } else {
                    id<MTLBlitCommandEncoder> blit = [cmdBuf blitCommandEncoder];
                    [blit copyFromTexture:privateOut
                              sourceSlice:0
                              sourceLevel:0
                             sourceOrigin:MTLOriginMake(0, 0, 0)
                               sourceSize:MTLSizeMake(outW, outH, 1)
                                toTexture:realTex
                         destinationSlice:0
                         destinationLevel:0
                        destinationOrigin:MTLOriginMake(0, 0, 0)];
                    [blit endEncoding];
                }
            }
            scaledEncoded = YES;
        }
    } else if (casPipeline && inW >= 64 && inH >= 64 && outW >= 64 && outH >= 64) {
        if (levi_encode_cas(cmdBuf, casPipeline, inputTex, realTex, g_sharpness)) {
            sharpenedEncoded = YES;
        }
    }

    if (!scaledEncoded && !sharpenedEncoded) {
        NSUInteger copyW = inW < outW ? inW : outW;
        NSUInteger copyH = inH < outH ? inH : outH;
        if (copyW > 0 && copyH > 0) {
            id<MTLBlitCommandEncoder> blit = [cmdBuf blitCommandEncoder];
            [blit copyFromTexture:inputTex
                      sourceSlice:0
                      sourceLevel:0
                     sourceOrigin:MTLOriginMake(0, 0, 0)
                       sourceSize:MTLSizeMake(copyW, copyH, 1)
                        toTexture:realTex
                 destinationSlice:0
                 destinationLevel:0
                destinationOrigin:MTLOriginMake(0, 0, 0)];
            [blit endEncoding];
        }
    }

    NSArray<MTLDrawablePresentedHandler> *handlers = nil;
    @synchronized(scaled) {
        if (scaled.handlers.count > 0) {
            handlers = [scaled.handlers copy];
            [scaled.handlers removeAllObjects];
        }
    }
    for (MTLDrawablePresentedHandler block in handlers) {
        [realDrawable addPresentedHandler:block];
    }

    @synchronized(state) {
        if (scaledEncoded && (!state.loggedSpatial ||
                              state.loggedInW != inW || state.loggedInH != inH ||
                              state.loggedOutW != outW || state.loggedOutH != outH)) {
            state.loggedSpatial = YES;
            state.loggedInW = inW;
            state.loggedInH = inH;
            state.loggedOutW = outW;
            state.loggedOutH = outH;
            fprintf(stderr, "LeviMetalFX: MTLFXSpatialScaler active %lux%lu -> %lux%lu (format=%lu, directOutput=%d, sharpness=%.2f)\n",
                    (unsigned long)inW, (unsigned long)inH,
                    (unsigned long)outW, (unsigned long)outH,
                    (unsigned long)outFmt, directOutput ? 1 : 0,
                    sharpenedEncoded ? (double)g_sharpness : 0.0);
            fflush(stderr);
        } else if (!scaledEncoded && sharpenedEncoded && (!state.loggedSharpenOnly ||
                   state.loggedOutW != outW || state.loggedOutH != outH)) {
            state.loggedSharpenOnly = YES;
            state.loggedOutW = outW;
            state.loggedOutH = outH;
            fprintf(stderr, "LeviMetalFX: CAS sharpen-only active %lux%lu -> %lux%lu (format=%lu, sharpness=%.2f)\n",
                    (unsigned long)inW, (unsigned long)inH,
                    (unsigned long)outW, (unsigned long)outH,
                    (unsigned long)outFmt, (double)g_sharpness);
            fflush(stderr);
        }
    }

    return realDrawable;
}

@interface LeviPendingPresent : NSObject
@property (nonatomic, strong) LeviScaledDrawable *drawable;
@property (nonatomic) int mode; /* 0 = present, 1 = atTime, 2 = afterMinimumDuration */
@property (nonatomic) CFTimeInterval timeValue;
@end

@implementation LeviPendingPresent
@end

static void levi_queue_pending_present(id cmdBuf, LeviScaledDrawable *drawable, int mode, CFTimeInterval timeValue) {
    LeviPendingPresent *entry = [LeviPendingPresent new];
    entry.drawable = drawable;
    entry.mode = mode;
    entry.timeValue = timeValue;
    @synchronized(cmdBuf) {
        NSMutableArray<LeviPendingPresent *> *list = objc_getAssociatedObject(cmdBuf, kLeviPendingPresentsKey);
        if (!list) {
            list = [NSMutableArray arrayWithCapacity:1];
            objc_setAssociatedObject(cmdBuf, kLeviPendingPresentsKey, list, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
        }
        [list addObject:entry];
    }
}

static void levi_flush_pending_presents(id cmdBuf) {
    NSArray<LeviPendingPresent *> *pending = nil;
    @synchronized(cmdBuf) {
        NSMutableArray<LeviPendingPresent *> *list = objc_getAssociatedObject(cmdBuf, kLeviPendingPresentsKey);
        if (list.count > 0) {
            pending = [list copy];
            [list removeAllObjects];
        }
    }
    if (pending.count == 0) return;
    id<MTLCommandBuffer> cb = (id<MTLCommandBuffer>)cmdBuf;
    for (LeviPendingPresent *entry in pending) {
        id<CAMetalDrawable> real = levi_encode_and_acquire_real(entry.drawable, cb);
        if (!real) continue;
        if (entry.mode == 1) {
            [cb presentDrawable:real atTime:entry.timeValue];
        } else if (entry.mode == 2) {
            [cb presentDrawable:real afterMinimumDuration:entry.timeValue];
        } else {
            [cb presentDrawable:real];
        }
    }
}

static Class levi_defining_class(Class cls, SEL sel) {
    Class owner = cls;
    while (owner) {
        unsigned int count = 0;
        Method *methods = class_copyMethodList(owner, &count);
        BOOL owned = NO;
        for (unsigned int i = 0; i < count; ++i) {
            if (method_getName(methods[i]) == sel) {
                owned = YES;
                break;
            }
        }
        free(methods);
        if (owned) return owner;
        owner = class_getSuperclass(owner);
    }
    return Nil;
}

static BOOL levi_should_bypass_present_delay(id<MTLDrawable> drawable) {
    if (g_vsync_override != 0 || !drawable) return NO;
    if ([drawable isKindOfClass:[LeviScaledDrawable class]]) {
        return YES;
    }
    if ([drawable respondsToSelector:@selector(layer)]) {
        CAMetalLayer *layer = [(id<CAMetalDrawable>)drawable layer];
        if ([layer isKindOfClass:[CAMetalLayer class]] && levi_is_wine_metal_layer(layer)) {
            return YES;
        }
    }
    return NO;
}

static void levi_log_present_bypass_once(const char *selectorName, CFTimeInterval value) {
    if (__atomic_exchange_n(&g_logged_present_bypass, 1, __ATOMIC_RELAXED) == 0) {
        fprintf(stderr, "LeviMetalFX: bypassed timed present (%s value=%.4f) because vsyncOverride=0\n",
                selectorName, (double)value);
        fflush(stderr);
    }
}

static void levi_swizzle_cmdbuf_selector(Class cls, SEL sel, int kind) {
    Class owner = levi_defining_class(cls, sel);
    if (!owner) return;
    NSString *key = [NSString stringWithFormat:@"%@::%s", NSStringFromClass(owner), sel_getName(sel)];
    @synchronized(g_swizzled_cmdbuf_methods) {
        if ([g_swizzled_cmdbuf_methods containsObject:key]) return;
        Method ownMethod = class_getInstanceMethod(owner, sel);
        if (!ownMethod) return;
        /* Capture this exact owner's original IMP inside a per-owner block so
         * subclass -> [super ...] chains never recurse back into the subclass. */
        IMP orig = method_getImplementation(ownMethod);
        if (!orig) return;
        IMP replacement = NULL;
        switch (kind) {
        case 0:
            replacement = imp_implementationWithBlock(^(id self, id<MTLDrawable> drawable) {
                if ([drawable isKindOfClass:[LeviScaledDrawable class]]) {
                    levi_queue_pending_present(self, (LeviScaledDrawable *)drawable, 0, 0.0);
                    return;
                }
                ((PresentDrawableFn)orig)(self, sel, drawable);
            });
            break;
        case 1:
            replacement = imp_implementationWithBlock(^(id self, id<MTLDrawable> drawable, CFTimeInterval time) {
                if ([drawable isKindOfClass:[LeviScaledDrawable class]]) {
                    if (g_vsync_override == 0) {
                        levi_log_present_bypass_once("presentDrawable:atTime:", time);
                        levi_queue_pending_present(self, (LeviScaledDrawable *)drawable, 0, 0.0);
                        return;
                    }
                    levi_queue_pending_present(self, (LeviScaledDrawable *)drawable, 1, time);
                    return;
                }
                if (levi_should_bypass_present_delay(drawable)) {
                    levi_log_present_bypass_once("presentDrawable:atTime:", time);
                    [(id<MTLCommandBuffer>)self presentDrawable:drawable];
                    return;
                }
                ((PresentDrawableAtTimeFn)orig)(self, sel, drawable, time);
            });
            break;
        case 2:
            replacement = imp_implementationWithBlock(^(id self, id<MTLDrawable> drawable, CFTimeInterval duration) {
                if ([drawable isKindOfClass:[LeviScaledDrawable class]]) {
                    if (g_vsync_override == 0) {
                        levi_log_present_bypass_once("presentDrawable:afterMinimumDuration:", duration);
                        levi_queue_pending_present(self, (LeviScaledDrawable *)drawable, 0, 0.0);
                        return;
                    }
                    levi_queue_pending_present(self, (LeviScaledDrawable *)drawable, 2, duration);
                    return;
                }
                if (levi_should_bypass_present_delay(drawable)) {
                    levi_log_present_bypass_once("presentDrawable:afterMinimumDuration:", duration);
                    [(id<MTLCommandBuffer>)self presentDrawable:drawable];
                    return;
                }
                ((PresentDrawableAfterMinFn)orig)(self, sel, drawable, duration);
            });
            break;
        case 3:
            replacement = imp_implementationWithBlock(^(id self) {
                levi_flush_pending_presents(self);
                ((CommitFn)orig)(self, sel);
            });
            break;
        default:
            return;
        }
        if (replacement) {
            method_setImplementation(ownMethod, replacement);
            [g_swizzled_cmdbuf_methods addObject:key];
        }
    }
}

static void levi_swizzle_drawable_selector(Class cls, SEL sel) {
    Class owner = levi_defining_class(cls, sel);
    if (!owner || owner == [LeviScaledDrawable class]) return;
    NSString *key = [NSString stringWithFormat:@"%@::%s", NSStringFromClass(owner), sel_getName(sel)];
    @synchronized(g_swizzled_drawable_methods) {
        if ([g_swizzled_drawable_methods containsObject:key]) return;
        Method ownMethod = class_getInstanceMethod(owner, sel);
        if (!ownMethod) return;
        IMP orig = method_getImplementation(ownMethod);
        if (!orig) return;
        const char *selName = sel_getName(sel);
        IMP replacement = imp_implementationWithBlock(^(id self, CFTimeInterval value) {
            if (levi_should_bypass_present_delay((id<MTLDrawable>)self)) {
                levi_log_present_bypass_once(selName, value);
                [(id<MTLDrawable>)self present];
                return;
            }
            ((DrawablePresentTimeFn)orig)(self, sel, value);
        });
        if (replacement) {
            method_setImplementation(ownMethod, replacement);
            [g_swizzled_drawable_methods addObject:key];
        }
    }
}

static void levi_swizzle_drawable_class(Class cls) {
    if (!cls || cls == [LeviScaledDrawable class]) return;
    static void *g_last_drawable_class = NULL;
    if ((__bridge void *)cls == __atomic_load_n(&g_last_drawable_class, __ATOMIC_RELAXED)) {
        return;
    }
    levi_ensure_tables();
    NSString *name = NSStringFromClass(cls);
    @synchronized(g_swizzled_drawable_classes) {
        if ([g_swizzled_drawable_classes containsObject:name]) {
            __atomic_store_n(&g_last_drawable_class, (__bridge void *)cls, __ATOMIC_RELAXED);
            return;
        }
        [g_swizzled_drawable_classes addObject:name];
    }
    levi_swizzle_drawable_selector(cls, @selector(presentAtTime:));
    levi_swizzle_drawable_selector(cls, @selector(presentAfterMinimumDuration:));
    __atomic_store_n(&g_last_drawable_class, (__bridge void *)cls, __ATOMIC_RELAXED);
    if (g_debug_log) {
        fprintf(stderr, "LeviMetalFX: hooked drawable class %s\n", name.UTF8String);
        fflush(stderr);
    }
}

static void levi_swizzle_command_buffer_class(Class cls) {
    if (!cls) return;
    levi_ensure_tables();
    NSString *name = NSStringFromClass(cls);
    @synchronized(g_swizzled_cmdbuf_classes) {
        if ([g_swizzled_cmdbuf_classes containsObject:name]) return;
        [g_swizzled_cmdbuf_classes addObject:name];
    }
    levi_swizzle_cmdbuf_selector(cls, @selector(presentDrawable:), 0);
    levi_swizzle_cmdbuf_selector(cls, @selector(presentDrawable:atTime:), 1);
    levi_swizzle_cmdbuf_selector(cls, @selector(presentDrawable:afterMinimumDuration:), 2);
    levi_swizzle_cmdbuf_selector(cls, @selector(commit), 3);
    if (g_debug_log) {
        fprintf(stderr, "LeviMetalFX: hooked command buffer class %s\n", name.UTF8String);
        fflush(stderr);
    }
}

static void levi_hook_command_buffers_for_device(id<MTLDevice> device) {
    if (!device) return;
    static void *g_last_probed_device = NULL;
    if ((__bridge void *)device == __atomic_load_n(&g_last_probed_device, __ATOMIC_RELAXED)) {
        return;
    }
    static NSMutableSet<NSNumber *> *probedDevices = nil;
    static dispatch_once_t once;
    dispatch_once(&once, ^{ probedDevices = [NSMutableSet new]; });
    NSNumber *key = @((uintptr_t)(__bridge void *)device);
    @synchronized(probedDevices) {
        if ([probedDevices containsObject:key]) {
            __atomic_store_n(&g_last_probed_device, (__bridge void *)device, __ATOMIC_RELAXED);
            return;
        }
        [probedDevices addObject:key];
    }
    @autoreleasepool {
        id<MTLCommandQueue> q = [device newCommandQueue];
        if (q) {
            id<MTLCommandBuffer> cb = [q commandBuffer];
            if (cb) {
                levi_swizzle_command_buffer_class(object_getClass(cb));
            }
            id<MTLCommandBuffer> cbUnretained = [q commandBufferWithUnretainedReferences];
            if (cbUnretained) {
                levi_swizzle_command_buffer_class(object_getClass(cbUnretained));
            }
        }
    }
    __atomic_store_n(&g_last_probed_device, (__bridge void *)device, __ATOMIC_RELAXED);
}

static void levi_pace_frame(LeviSpatialLayerState *state) {
    uint64_t interval_ns = g_frame_interval_ns;
    if (!state || interval_ns == 0) return;

    uint64_t now_ns = clock_gettime_nsec_np(CLOCK_UPTIME_RAW);
    uint64_t deadline_ns = 0;
    uint64_t sleep_ns = 0;

    @synchronized(state) {
        if (state.nextFrameDeadlineNs == 0) {
            state.nextFrameDeadlineNs = now_ns + interval_ns;
        } else if (now_ns < state.nextFrameDeadlineNs) {
            deadline_ns = state.nextFrameDeadlineNs;
            sleep_ns = deadline_ns - now_ns;
            state.nextFrameDeadlineNs = deadline_ns + interval_ns;
        } else {
            /* Missed interval: reset cadence without triggering a catch-up burst. */
            state.nextFrameDeadlineNs = now_ns + interval_ns;
        }
    }

    if (sleep_ns == 0) return;
    if (sleep_ns > 1000000000ULL) {
        sleep_ns = 1000000000ULL;
        deadline_ns = now_ns + sleep_ns;
    }

    /* Coarse nanosleep leaving ~1.0ms margin to absorb macOS timer coalescing. */
    const uint64_t spin_margin_ns = 1000000ULL;
    if (sleep_ns > spin_margin_ns + 200000ULL) {
        uint64_t coarse_ns = sleep_ns - spin_margin_ns;
        struct timespec req;
        req.tv_sec = (time_t)(coarse_ns / 1000000000ULL);
        req.tv_nsec = (long)(coarse_ns % 1000000000ULL);
        nanosleep(&req, NULL);
    }

    /* Fine tail wait to lock frame cadence to sub-millisecond accuracy. */
    while (clock_gettime_nsec_np(CLOCK_UPTIME_RAW) < deadline_ns) {
#if defined(__x86_64__)
        __builtin_ia32_pause();
#else
        sched_yield();
#endif
    }
}

static BOOL levi_image_has_opaque_corners(NSBitmapImageRep *rep) {
    if (!rep || rep.pixelsWide < 4 || rep.pixelsHigh < 4) return NO;
    NSInteger w = rep.pixelsWide;
    NSInteger h = rep.pixelsHigh;
    NSInteger xs[4] = {1, w - 2, 1, w - 2};
    NSInteger ys[4] = {1, 1, h - 2, h - 2};
    for (int i = 0; i < 4; ++i) {
        NSColor *c = [[rep colorAtX:xs[i] y:ys[i]] colorUsingColorSpace:[NSColorSpace sRGBColorSpace]];
        if (!c || c.alphaComponent < 0.96) return NO;
    }
    return YES;
}

static NSRect levi_aspect_rect(NSSize srcSize, NSRect dstRect, BOOL fill) {
    if (srcSize.width <= 0.0 || srcSize.height <= 0.0) return dstRect;
    CGFloat sx = dstRect.size.width / srcSize.width;
    CGFloat sy = dstRect.size.height / srcSize.height;
    CGFloat scale = fill ? (sx > sy ? sx : sy) : (sx < sy ? sx : sy);
    CGFloat w = srcSize.width * scale;
    CGFloat h = srcSize.height * scale;
    return NSMakeRect(dstRect.origin.x + (dstRect.size.width - w) * 0.5,
                      dstRect.origin.y + (dstRect.size.height - h) * 0.5,
                      w, h);
}

static NSImage *levi_load_dock_icon(void) {
    dispatch_once(&g_dock_icon_once, ^{
        if (!g_game_icon_path || g_game_icon_path.length == 0) return;

        NSImage *source = [[NSImage alloc] initWithContentsOfFile:g_game_icon_path];
        if (!source || source.size.width <= 0 || source.size.height <= 0) {
            if (g_debug_log) {
                fprintf(stderr, "LeviMetalFX: failed to load icon from %s\n", g_game_icon_path.UTF8String);
                fflush(stderr);
            }
            return;
        }

        /* Probe corner opacity to decide whether to apply a macOS squircle mask. */
        BOOL opaqueSquare = NO;
        NSData *tiff = source.TIFFRepresentation;
        if (tiff) {
            NSBitmapImageRep *probeRep = [NSBitmapImageRep imageRepWithData:tiff];
            opaqueSquare = levi_image_has_opaque_corners(probeRep);
        }

        const size_t canvasPx = 512;
        const CGFloat canvas = (CGFloat)canvasPx;
        CGColorSpaceRef cs = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
        if (!cs) {
            g_cached_dock_icon = source;
            return;
        }
        CGContextRef cgCtx = CGBitmapContextCreate(NULL, canvasPx, canvasPx, 8, canvasPx * 4,
                                                   cs, (CGBitmapInfo)kCGImageAlphaPremultipliedLast);
        CGColorSpaceRelease(cs);
        if (!cgCtx) {
            g_cached_dock_icon = source;
            return;
        }

        NSGraphicsContext *ctx = [NSGraphicsContext graphicsContextWithCGContext:cgCtx flipped:NO];
        if (!ctx) {
            CGContextRelease(cgCtx);
            g_cached_dock_icon = source;
            return;
        }

        [NSGraphicsContext saveGraphicsState];
        [NSGraphicsContext setCurrentContext:ctx];
        ctx.imageInterpolation = NSImageInterpolationHigh;
        CGContextClearRect(cgCtx, CGRectMake(0, 0, canvas, canvas));

        if (opaqueSquare) {
            NSRect iconRect = NSMakeRect(50.0, 50.0, 412.0, 412.0);
            CGFloat radius = 92.0;
            NSBezierPath *squircle = [NSBezierPath bezierPathWithRoundedRect:iconRect xRadius:radius yRadius:radius];
            NSShadow *shadow = [NSShadow new];
            shadow.shadowColor = [[NSColor blackColor] colorWithAlphaComponent:0.28];
            shadow.shadowOffset = NSMakeSize(0.0, -8.0);
            shadow.shadowBlurRadius = 18.0;
            [NSGraphicsContext saveGraphicsState];
            [shadow set];
            [[NSColor blackColor] setFill];
            [squircle fill];
            [NSGraphicsContext restoreGraphicsState];

            [NSGraphicsContext saveGraphicsState];
            [squircle addClip];
            NSRect drawRect = levi_aspect_rect(source.size, iconRect, YES);
            [source drawInRect:drawRect fromRect:NSZeroRect operation:NSCompositingOperationSourceOver fraction:1.0];
            [NSGraphicsContext restoreGraphicsState];
        } else {
            NSRect iconRect = NSMakeRect(44.0, 44.0, 424.0, 424.0);
            NSRect drawRect = levi_aspect_rect(source.size, iconRect, NO);
            [source drawInRect:drawRect fromRect:NSZeroRect operation:NSCompositingOperationSourceOver fraction:1.0];
        }

        [NSGraphicsContext restoreGraphicsState];

        CGImageRef cgImg = CGBitmapContextCreateImage(cgCtx);
        CGContextRelease(cgCtx);
        if (cgImg) {
            g_cached_dock_icon = [[NSImage alloc] initWithCGImage:cgImg size:NSMakeSize(canvas, canvas)];
            CGImageRelease(cgImg);
        } else {
            g_cached_dock_icon = source;
        }
        if (g_debug_log) {
            fprintf(stderr, "LeviMetalFX: loaded Dock icon %s (squircle=%d)\n", g_game_icon_path.UTF8String, opaqueSquare ? 1 : 0);
            fflush(stderr);
        }
    });
    return g_cached_dock_icon;
}

static BOOL levi_is_wine_game_window(NSWindow *window, NSString *incomingTitle) {
    if (!window) return NO;
    NSString *clsName = NSStringFromClass([window class]);
    if (![clsName containsString:@"WineWindow"] && ![clsName containsString:@"LeviProbeWindow"]) {
        return NO;
    }
    if (window.parentWindow != nil) return NO;
    /* Keep Win32 MessageBox / sign-in prompts with their real titles. */
    if (incomingTitle && [incomingTitle localizedCaseInsensitiveContainsString:@"sign-in"]) {
        return NO;
    }
    if (incomingTitle && incomingTitle.length > 0) {
        if ([incomingTitle localizedCaseInsensitiveCompare:@"Minecraft"] == NSOrderedSame ||
            [[incomingTitle lowercaseString] hasPrefix:@"minecraft "]) {
            return YES;
        }
    }
    NSWindowStyleMask style = window.styleMask;
    CGSize sz = window.frame.size;
    return (style & NSWindowStyleMaskResizable) != 0 && sz.width >= 400.0 && sz.height >= 250.0;
}

static void levi_setApplicationIconImage(id self, SEL _cmd, NSImage *image) {
    NSImage *custom = levi_load_dock_icon();
    if (custom) {
        image = custom;
    }
    if (g_orig_setAppIcon) {
        ((SetAppIconFn)g_orig_setAppIcon)(self, _cmd, image);
    }
}

static void levi_setWindowTitle(id self, SEL _cmd, NSString *title) {
    NSWindow *window = (NSWindow *)self;
    if (g_game_title && g_game_title.length > 0 && levi_is_wine_game_window(window, title)) {
        title = g_game_title;
    }
    if (g_orig_setWindowTitle) {
        ((SetWindowTitleFn)g_orig_setWindowTitle)(self, _cmd, title);
    }
}

static void levi_brand_app_menu(NSMenu *menu) {
    NSMenuItem *appItem = menu.itemArray.firstObject;
    if (appItem.submenu && [appItem.title caseInsensitiveCompare:@"Wine"] == NSOrderedSame) {
        appItem.title = g_game_title;
        appItem.submenu.title = g_game_title;
    }
}

static void levi_setMainMenu(id self, SEL selector, NSMenu *menu) {
    ((SetMainMenuFn)g_orig_setMainMenu)(self, selector, menu);
    levi_brand_app_menu(menu);
}

static int g_function_key_guard = -1;
static io_connect_t g_keyboard_parameters = IO_OBJECT_NULL;
static int g_previous_function_key_mode;

static int levi_watch_function_keys(const char *host, int previousMode) {
    int pipeFD[2];
    if (!host || host[0] != '/' || pipe(pipeFD) != 0) return -1;
    if (fcntl(pipeFD[1], F_SETFD, FD_CLOEXEC) < 0 ||
        fcntl(pipeFD[1], F_SETNOSIGPIPE, 1) < 0) {
        close(pipeFD[0]);
        close(pipeFD[1]);
        return -1;
    }
    posix_spawn_file_actions_t actions;
    int status = posix_spawn_file_actions_init(&actions);
    if (status == 0) {
        status = posix_spawn_file_actions_adddup2(&actions, pipeFD[0], STDIN_FILENO);
        if (status == 0 && pipeFD[0] != STDIN_FILENO) {
            status = posix_spawn_file_actions_addclose(&actions, pipeFD[0]);
        }
        if (status == 0) {
            char mode[32];
            snprintf(mode, sizeof(mode), "%d", previousMode);
            char *args[] = {(char *)host, "--restore-function-keys", mode, NULL};
            char *env[] = {NULL}; // The recovery process does not load the Wine dylib.
            pid_t pid;
            status = posix_spawn(&pid, host, &actions, NULL, args, env);
            if (status == 0) {
                dispatch_async(dispatch_get_global_queue(QOS_CLASS_UTILITY, 0), ^{
                    while (waitpid(pid, NULL, 0) < 0 && errno == EINTR) {}
                });
            }
        }
        posix_spawn_file_actions_destroy(&actions);
    }
    close(pipeFD[0]);
    if (status == 0) return pipeFD[1];
    close(pipeFD[1]);
    return -1;
}

static void levi_restore_function_keys(void) {
    if (g_function_key_guard < 0) return;
    if (levi_set_function_key_mode(g_keyboard_parameters, g_previous_function_key_mode)) {
        ssize_t written;
        do { written = write(g_function_key_guard, "r", 1); } while (written < 0 && errno == EINTR);
    }
    close(g_function_key_guard);
    g_function_key_guard = -1;
    IOServiceClose(g_keyboard_parameters);
    g_keyboard_parameters = IO_OBJECT_NULL;
}

static void levi_update_function_keys(void) {
    if (!NSApp.isActive || !levi_is_wine_game_window(NSApp.keyWindow, NSApp.keyWindow.title)) {
        levi_restore_function_keys();
        return;
    }
    if (g_function_key_guard >= 0) return;
    io_connect_t connection = levi_open_keyboard_parameters();
    if (!connection) return;
    int previousMode;
    if (levi_read_function_key_mode(connection, &previousMode) && previousMode != 1) {
        int guard = levi_watch_function_keys(getenv("LEVI_GAME_HOST_BIN"), previousMode);
        if (guard >= 0) {
            if (levi_set_function_key_mode(connection, 1)) {
                g_function_key_guard = guard;
                g_keyboard_parameters = connection;
                g_previous_function_key_mode = previousMode;
                return;
            }
            close(guard);
        }
    }
    IOServiceClose(connection);
}

static void levi_apply_app_branding_on_main(void) {
    if (!g_game_icon_path && !g_game_title) return;
    void (^applyBlock)(void) = ^{
        if (g_game_icon_path && NSApp && g_orig_setAppIcon) {
            NSImage *icon = levi_load_dock_icon();
            if (icon) {
                ((SetAppIconFn)g_orig_setAppIcon)(NSApp, @selector(setApplicationIconImage:), icon);
            }
        }
        if (g_game_title.length > 0 && NSApp && g_orig_setWindowTitle) {
            levi_brand_app_menu(NSApp.mainMenu);
            for (NSWindow *w in NSApp.windows) {
                if (levi_is_wine_game_window(w, w.title)) {
                    ((SetWindowTitleFn)g_orig_setWindowTitle)(w, @selector(setTitle:), g_game_title);
                }
            }
            if (getenv("LEVI_GAME_HOST_BIN")) levi_update_function_keys();
        }
    };
    if ([NSThread isMainThread]) {
        applyBlock();
    } else {
        dispatch_async(dispatch_get_main_queue(), applyBlock);
    }
}

static void levi_apply_surface_policy(CAMetalLayer *layer, LeviSpatialLayerState *state) {
    if (g_opaque_policy) {
        if (g_orig_setOpaque && !layer.isOpaque) {
            ((SetBoolFn)g_orig_setOpaque)(layer, @selector(setOpaque:), YES);
        }
        if (g_orig_setPresentsWithTransaction && layer.presentsWithTransaction) {
            ((SetBoolFn)g_orig_setPresentsWithTransaction)(layer, @selector(setPresentsWithTransaction:), NO);
        }
    }
    if (g_vsync_override >= 0 && g_orig_setDisplaySyncEnabled) {
        BOOL wantSync = (g_vsync_override == 1);
        if (layer.displaySyncEnabled != wantSync) {
            ((SetBoolFn)g_orig_setDisplaySyncEnabled)(layer, @selector(setDisplaySyncEnabled:), wantSync);
        }
    }
    if (g_drawable_count >= 2 && g_drawable_count <= 3 && g_orig_setMaximumDrawableCount) {
        if (layer.maximumDrawableCount != g_drawable_count) {
            ((SetNSUIntegerFn)g_orig_setMaximumDrawableCount)(layer, @selector(setMaximumDrawableCount:), g_drawable_count);
        }
    }
    if (state && !state.loggedSurface) {
        @synchronized(state) {
            if (!state.loggedSurface) {
                state.loggedSurface = YES;
                levi_apply_app_branding_on_main();
                fprintf(stderr, "LeviMetalFX: surface configured opaque=%d transaction=%d displaySync=%d drawableCount=%lu vsyncOverride=%d maxFps=%d pixelFormat=%lu factor=%.2f sharpness=%.2f sharpenOnly=%d\n",
                        layer.isOpaque ? 1 : 0,
                        layer.presentsWithTransaction ? 1 : 0,
                        layer.displaySyncEnabled ? 1 : 0,
                        (unsigned long)layer.maximumDrawableCount,
                        g_vsync_override,
                        g_max_fps,
                        (unsigned long)layer.pixelFormat,
                        g_spatial_factor,
                        (double)g_sharpness,
                        g_sharpen_only ? 1 : 0);
                fflush(stderr);
            }
        }
    }
}

static void levi_setDisplaySyncEnabled(id self, SEL _cmd, BOOL flag) {
    CAMetalLayer *layer = (CAMetalLayer *)self;
    if (g_vsync_override >= 0 && levi_is_wine_metal_layer(layer)) {
        flag = (g_vsync_override == 1);
    }
    if (g_orig_setDisplaySyncEnabled) {
        ((SetBoolFn)g_orig_setDisplaySyncEnabled)(self, _cmd, flag);
    }
}

static void levi_setMaximumDrawableCount(id self, SEL _cmd, NSUInteger count) {
    CAMetalLayer *layer = (CAMetalLayer *)self;
    if (g_drawable_count >= 2 && g_drawable_count <= 3 && levi_is_wine_metal_layer(layer)) {
        count = g_drawable_count;
    }
    if (g_orig_setMaximumDrawableCount) {
        ((SetNSUIntegerFn)g_orig_setMaximumDrawableCount)(self, _cmd, count);
    }
}

static void levi_setOpaque(id self, SEL _cmd, BOOL opaque) {
    CAMetalLayer *layer = (CAMetalLayer *)self;
    if (g_opaque_policy && levi_is_wine_metal_layer(layer)) {
        opaque = YES;
    }
    if (g_orig_setOpaque) {
        ((SetBoolFn)g_orig_setOpaque)(self, _cmd, opaque);
    }
}

static void levi_setPresentsWithTransaction(id self, SEL _cmd, BOOL flag) {
    CAMetalLayer *layer = (CAMetalLayer *)self;
    if (g_opaque_policy && levi_is_wine_metal_layer(layer)) {
        flag = NO;
    }
    if (g_orig_setPresentsWithTransaction) {
        ((SetBoolFn)g_orig_setPresentsWithTransaction)(self, _cmd, flag);
    }
}

static void levi_setFramebufferOnly(id self, SEL _cmd, BOOL flag) {
    CAMetalLayer *layer = (CAMetalLayer *)self;
    if (levi_spatial_enabled() && levi_is_wine_metal_layer(layer)) {
        flag = NO;
    }
    if (g_orig_setFramebufferOnly) {
        ((SetBoolFn)g_orig_setFramebufferOnly)(self, _cmd, flag);
    }
}

static void levi_setContentsScale(id self, SEL _cmd, CGFloat scale) {
    if (g_retina_upscale && [self isKindOfClass:[CAMetalLayer class]]) {
        CAMetalLayer *layer = (CAMetalLayer *)self;
        if (levi_is_wine_metal_layer(layer)) {
            CGFloat desired = levi_desired_backing_scale(layer);
            if (desired > scale) {
                scale = desired;
            }
        }
    }
    if (g_orig_setContentsScale) {
        ((SetCGFloatFn)g_orig_setContentsScale)(self, _cmd, scale);
    }
}

static void levi_setDrawableSize(id self, SEL _cmd, CGSize size) {
    CAMetalLayer *layer = (CAMetalLayer *)self;
    if (levi_spatial_enabled() && levi_is_wine_metal_layer(layer)) {
        LeviSpatialLayerState *state = levi_state_for_layer(layer);
        CGSize nativeSize = levi_native_output_size(layer, state.lastOutputSize);
        if (nativeSize.width >= 64.0 && nativeSize.height >= 64.0) {
            @synchronized(state) {
                state.lastOutputSize = nativeSize;
                if (size.width >= 64.0 && size.height >= 64.0 &&
                    size.width < nativeSize.width * 0.96 &&
                    size.height < nativeSize.height * 0.96) {
                    /* D3DMetal / Wine requested the logical (pre-upscale) size. */
                    state.requestedInputSize = size;
                } else {
                    /* Native resize from Wine / AppKit. */
                    state.lastOutputSize = size;
                    state.requestedInputSize = CGSizeZero;
                }
            }
            if (g_orig_setDrawableSize) {
                ((SetDrawableSizeFn)g_orig_setDrawableSize)(self, _cmd, nativeSize);
            }
            return;
        }
    }
    if (g_orig_setDrawableSize) {
        ((SetDrawableSizeFn)g_orig_setDrawableSize)(self, _cmd, size);
    }
}

static id<CAMetalDrawable> levi_nextDrawable(id self, SEL _cmd) {
    CAMetalLayer *layer = (CAMetalLayer *)self;
    if (!levi_is_wine_metal_layer(layer)) {
        return g_orig_nextDrawable ? ((NextDrawableFn)g_orig_nextDrawable)(self, _cmd) : nil;
    }

    LeviSpatialLayerState *state = levi_state_for_layer(layer);
    levi_apply_surface_policy(layer, state);
    levi_pace_frame(state);

    BOOL spatialMode = levi_spatial_enabled();
    BOOL sharpenOnlyMode = levi_sharpen_only_enabled();
    if (!spatialMode && !sharpenOnlyMode) {
        if (g_vsync_override == 0) {
            levi_hook_command_buffers_for_device(layer.device);
        }
        id<CAMetalDrawable> realDrawable = g_orig_nextDrawable ? ((NextDrawableFn)g_orig_nextDrawable)(self, _cmd) : nil;
        if (g_vsync_override == 0 && realDrawable) {
            levi_swizzle_drawable_class(object_getClass(realDrawable));
        }
        return realDrawable;
    }

    id<MTLDevice> device = layer.device;
    if (!device) {
        return g_orig_nextDrawable ? ((NextDrawableFn)g_orig_nextDrawable)(self, _cmd) : nil;
    }
    if (spatialMode && ![MTLFXSpatialScalerDescriptor supportsDevice:device]) {
        return g_orig_nextDrawable ? ((NextDrawableFn)g_orig_nextDrawable)(self, _cmd) : nil;
    }

    NSUInteger inW = 0;
    NSUInteger inH = 0;
    if (spatialMode) {
        if (g_retina_upscale && g_orig_setContentsScale) {
            CGFloat desiredScale = levi_desired_backing_scale(layer);
            if (desiredScale > layer.contentsScale + 0.01) {
                ((SetCGFloatFn)g_orig_setContentsScale)(layer, @selector(setContentsScale:), desiredScale);
            }
        }

        CGSize outputSize = levi_native_output_size(layer, state.lastOutputSize);
        if (outputSize.width < 64.0 || outputSize.height < 64.0) {
            return g_orig_nextDrawable ? ((NextDrawableFn)g_orig_nextDrawable)(self, _cmd) : nil;
        }

        CGSize inputSize;
        @synchronized(state) {
            state.lastOutputSize = outputSize;
            if (g_retina_upscale && g_spatial_factor <= 1.01) {
                /* Non-Retina Wine window (1x logical points) -> native Retina backing (2x pixels). */
                CGRect b = layer.bounds;
                CGFloat lw = floor(b.size.width + 0.5);
                CGFloat lh = floor(b.size.height + 0.5);
                if (lw >= 64.0 && lh >= 64.0 && lw < outputSize.width && lh < outputSize.height) {
                    inputSize = CGSizeMake(lw, lh);
                } else if (state.requestedInputSize.width >= 64.0 && state.requestedInputSize.height >= 64.0 &&
                           state.requestedInputSize.width < outputSize.width &&
                           state.requestedInputSize.height < outputSize.height) {
                    inputSize = state.requestedInputSize;
                } else {
                    inputSize = outputSize;
                }
            } else if (state.requestedInputSize.width >= 64.0 && state.requestedInputSize.height >= 64.0 &&
                       state.requestedInputSize.width < outputSize.width &&
                       state.requestedInputSize.height < outputSize.height) {
                inputSize = state.requestedInputSize;
            } else {
                inputSize = levi_scaled_input_size(outputSize, g_spatial_factor);
            }
        }

        inW = (NSUInteger)inputSize.width;
        inH = (NSUInteger)inputSize.height;
        NSUInteger outW = (NSUInteger)outputSize.width;
        NSUInteger outH = (NSUInteger)outputSize.height;
        if (inW < 64 || inH < 64 || inW >= outW || inH >= outH) {
            return g_orig_nextDrawable ? ((NextDrawableFn)g_orig_nextDrawable)(self, _cmd) : nil;
        }

        /* Keep the underlying CAMetalLayer at native output resolution and writable. */
        CGSize currentDrawableSize = layer.drawableSize;
        if (g_orig_setDrawableSize &&
            (fabs(currentDrawableSize.width - outputSize.width) > 0.5 ||
             fabs(currentDrawableSize.height - outputSize.height) > 0.5)) {
            ((SetDrawableSizeFn)g_orig_setDrawableSize)(layer, @selector(setDrawableSize:), outputSize);
        }
        if (g_orig_setFramebufferOnly && layer.framebufferOnly) {
            ((SetBoolFn)g_orig_setFramebufferOnly)(layer, @selector(setFramebufferOnly:), NO);
        }
    } else {
        /* Sharpen-only mode (Mullion / VKD3D): keep renderer's native drawableSize untouched. */
        CGSize nativeSize = layer.drawableSize;
        if (nativeSize.width < 64.0 || nativeSize.height < 64.0) {
            nativeSize = levi_native_output_size(layer, state.lastOutputSize);
        }
        inW = (NSUInteger)floor(nativeSize.width + 0.5);
        inH = (NSUInteger)floor(nativeSize.height + 0.5);
        if (inW < 64 || inH < 64) {
            return g_orig_nextDrawable ? ((NextDrawableFn)g_orig_nextDrawable)(self, _cmd) : nil;
        }
    }

    levi_hook_command_buffers_for_device(device);

    MTLPixelFormat fmt = layer.pixelFormat != MTLPixelFormatInvalid ? layer.pixelFormat : MTLPixelFormatBGRA8Unorm;
    id<MTLTexture> inputTex = nil;
    @synchronized(state) {
        if (state.poolDevice != device ||
            state.poolWidth != inW || state.poolHeight != inH ||
            state.poolFormat != fmt || state.inputPool.count == 0) {
            [state.inputPool removeAllObjects];
            MTLTextureDescriptor *td = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:fmt
                                                                                          width:inW
                                                                                         height:inH
                                                                                      mipmapped:NO];
            td.storageMode = MTLStorageModePrivate;
            td.usage = MTLTextureUsageRenderTarget | MTLTextureUsageShaderRead | MTLTextureUsageShaderWrite | MTLTextureUsagePixelFormatView;
            for (int i = 0; i < 4; ++i) {
                id<MTLTexture> t = [device newTextureWithDescriptor:td];
                if (t) [state.inputPool addObject:t];
            }
            state.poolDevice = device;
            state.poolWidth = inW;
            state.poolHeight = inH;
            state.poolFormat = fmt;
            state.poolIndex = 0;
        }
        if (state.inputPool.count > 0) {
            inputTex = state.inputPool[state.poolIndex % state.inputPool.count];
            state.poolIndex = (state.poolIndex + 1) % state.inputPool.count;
        }
    }

    if (!inputTex) {
        return g_orig_nextDrawable ? ((NextDrawableFn)g_orig_nextDrawable)(self, _cmd) : nil;
    }

    NSUInteger did = (NSUInteger)__atomic_add_fetch(&g_drawable_seq, 1, __ATOMIC_RELAXED);
    return [[LeviScaledDrawable alloc] initWithTexture:inputTex
                                                 layer:layer
                                                 state:state
                                            drawableID:did];
}

__attribute__((constructor))
static void levi_metal_fx_init(void) {
    const char *prog = getprogname();
    if (prog && strcmp(prog, "wineserver") == 0) {
        return;
    }

    const char *opaque_env = getenv("LEVI_METAL_OPAQUE");
    if (opaque_env && strcmp(opaque_env, "0") == 0) {
        g_opaque_policy = NO;
    }
    const char *debug_env = getenv("LEVI_METALFX_DEBUG");
    if (debug_env && strcmp(debug_env, "1") == 0) {
        g_debug_log = YES;
    }
    const char *vsync_env = getenv("LEVI_METAL_VSYNC");
    if (vsync_env && vsync_env[0] != '\0') {
        if (strcmp(vsync_env, "0") == 0) g_vsync_override = 0;
        else if (strcmp(vsync_env, "1") == 0) g_vsync_override = 1;
    }
    const char *drawable_count_env = getenv("LEVI_METAL_DRAWABLE_COUNT");
    if (drawable_count_env && drawable_count_env[0] != '\0') {
        if (strcmp(drawable_count_env, "3") == 0) g_drawable_count = 3;
        else if (strcmp(drawable_count_env, "2") == 0) g_drawable_count = 2;
        else if (strcmp(drawable_count_env, "0") == 0) g_drawable_count = 0;
    }
    const char *fps_env = getenv("LEVI_METAL_MAX_FPS");
    if (fps_env && fps_env[0] != '\0') {
        char *end = NULL;
        long parsed = strtol(fps_env, &end, 10);
        if (end && end != fps_env && *end == '\0' && parsed >= 1 && parsed <= 1000) {
            g_max_fps = (int)parsed;
            g_frame_interval_ns = (uint64_t)llround(1000000000.0 / (double)g_max_fps);
        }
    }
    const char *d3dm_mtl4 = getenv("D3DM_MTL4");
    BOOL mtl4_active = (d3dm_mtl4 && strcmp(d3dm_mtl4, "1") == 0);
    const char *retina_up_env = getenv("LEVI_METALFX_RETINA_UPSCALE");
    if (!mtl4_active && retina_up_env && strcmp(retina_up_env, "1") == 0) {
        g_retina_upscale = YES;
        g_main_display_scale = levi_query_main_display_scale();
    }
    const char *factor_env = getenv("LEVI_METALFX_SPATIAL_FACTOR");
    if (!mtl4_active && factor_env && factor_env[0] != '\0') {
        char *end = NULL;
        double parsed = strtod(factor_env, &end);
        if (end && end != factor_env && isfinite(parsed) && parsed > 1.01 && parsed <= 4.0) {
            g_spatial_factor = parsed;
        }
    }
    const char *sharp_env = getenv("LEVI_METALFX_SHARPNESS");
    if (!mtl4_active && sharp_env && sharp_env[0] != '\0') {
        char *end = NULL;
        double parsed = strtod(sharp_env, &end);
        if (end && end != sharp_env && isfinite(parsed) && parsed > 0.0 && parsed <= 1.0) {
            g_sharpness = (float)parsed;
        }
    }
    const char *sharpen_only_env = getenv("LEVI_METALFX_SHARPEN_ONLY");
    if (!mtl4_active && sharpen_only_env && strcmp(sharpen_only_env, "1") == 0) {
        g_sharpen_only = YES;
    }

    const char *icon_env = getenv("LEVI_GAME_ICON_PATH");
    if (icon_env && icon_env[0] == '/') {
        g_game_icon_path = [NSString stringWithUTF8String:icon_env];
    }
    const char *title_env = getenv("LEVI_GAME_TITLE");
    if (title_env && title_env[0] != '\0') {
        g_game_title = [NSString stringWithUTF8String:title_env];
        if (g_game_title.length > 0) {
            // AppKit gets its application-menu title from the loader's
            // embedded bundle information. Update the cached name before
            // Wine initializes NSApplication.
            NSDictionary *info = NSBundle.mainBundle.infoDictionary;
            if ([info[@"CFBundleName"] isEqual:@"Wine"] &&
                [info isKindOfClass:[NSMutableDictionary class]]) {
                ((NSMutableDictionary *)info)[@"CFBundleName"] = g_game_title;
            }
            [NSProcessInfo processInfo].processName = g_game_title;
        }
    }

    Class appClass = NSClassFromString(@"NSApplication");
    if (appClass && g_game_title.length > 0) {
        Method menuMethod = class_getInstanceMethod(appClass, @selector(setMainMenu:));
        if (menuMethod) {
            g_orig_setMainMenu = method_setImplementation(menuMethod, (IMP)levi_setMainMenu);
        }
        if (getenv("LEVI_GAME_HOST_BIN")) {
            NSNotificationCenter *center = [NSNotificationCenter defaultCenter];
            for (NSNotificationName name in @[NSApplicationDidBecomeActiveNotification,
                                              NSWindowDidBecomeKeyNotification,
                                              NSWindowDidResignKeyNotification]) {
                [center addObserverForName:name object:nil queue:[NSOperationQueue mainQueue]
                                usingBlock:^(__unused NSNotification *note) { levi_update_function_keys(); }];
            }
            for (NSNotificationName name in @[NSApplicationDidResignActiveNotification,
                                              NSApplicationWillTerminateNotification]) {
                [center addObserverForName:name object:nil queue:[NSOperationQueue mainQueue]
                                usingBlock:^(__unused NSNotification *note) { levi_restore_function_keys(); }];
            }
        }
    }
    if (appClass && g_game_icon_path) {
        Method am = class_getInstanceMethod(appClass, @selector(setApplicationIconImage:));
        if (am) {
            g_orig_setAppIcon = method_setImplementation(am, (IMP)levi_setApplicationIconImage);
        }
    }
    Class winClass = NSClassFromString(@"NSWindow");
    if (winClass && g_game_title) {
        Method wm = class_getInstanceMethod(winClass, @selector(setTitle:));
        if (wm) {
            g_orig_setWindowTitle = method_setImplementation(wm, (IMP)levi_setWindowTitle);
        }
    }
    if (g_game_icon_path || g_game_title) {
        [[NSNotificationCenter defaultCenter]
            addObserverForName:NSApplicationDidFinishLaunchingNotification
                        object:nil
                         queue:[NSOperationQueue mainQueue]
                    usingBlock:^(__unused NSNotification *note) {
                        levi_apply_app_branding_on_main();
                    }];
    }

    Class metalLayerClass = NSClassFromString(@"CAMetalLayer");
    if (!metalLayerClass) return;

    levi_ensure_tables();

    Method m;
    if ((m = class_getInstanceMethod(metalLayerClass, @selector(setDrawableSize:)))) {
        g_orig_setDrawableSize = method_setImplementation(m, (IMP)levi_setDrawableSize);
    }
    if (g_retina_upscale) {
        Class caLayerClass = NSClassFromString(@"CALayer");
        if (caLayerClass && (m = class_getInstanceMethod(caLayerClass, @selector(setContentsScale:)))) {
            g_orig_setContentsScale = method_setImplementation(m, (IMP)levi_setContentsScale);
        }
    }
    if ((m = class_getInstanceMethod(metalLayerClass, @selector(setOpaque:)))) {
        g_orig_setOpaque = method_setImplementation(m, (IMP)levi_setOpaque);
    }
    if ((m = class_getInstanceMethod(metalLayerClass, @selector(setPresentsWithTransaction:)))) {
        g_orig_setPresentsWithTransaction = method_setImplementation(m, (IMP)levi_setPresentsWithTransaction);
    }
    if ((m = class_getInstanceMethod(metalLayerClass, @selector(setDisplaySyncEnabled:)))) {
        g_orig_setDisplaySyncEnabled = method_setImplementation(m, (IMP)levi_setDisplaySyncEnabled);
    }
    if ((m = class_getInstanceMethod(metalLayerClass, @selector(setMaximumDrawableCount:)))) {
        g_orig_setMaximumDrawableCount = method_setImplementation(m, (IMP)levi_setMaximumDrawableCount);
    }
    if ((m = class_getInstanceMethod(metalLayerClass, @selector(setFramebufferOnly:)))) {
        g_orig_setFramebufferOnly = method_setImplementation(m, (IMP)levi_setFramebufferOnly);
    }
    if ((m = class_getInstanceMethod(metalLayerClass, @selector(nextDrawable)))) {
        g_orig_nextDrawable = method_setImplementation(m, (IMP)levi_nextDrawable);
    }

    /* Pre-swizzle Metal's base command buffer and CAMetalDrawable classes if available. */
    Class baseCmdBuf = NSClassFromString(@"_MTLCommandBuffer");
    if (baseCmdBuf) {
        levi_swizzle_command_buffer_class(baseCmdBuf);
    }
    Class metalDrawableClass = NSClassFromString(@"CAMetalDrawable");
    if (metalDrawableClass) {
        levi_swizzle_drawable_class(metalDrawableClass);
    }

    if (g_debug_log) {
        fprintf(stderr, "LeviMetalFX: loaded pid=%d opaque=%d vsync=%d maxFps=%d spatialFactor=%.2f sharpness=%.2f sharpenOnly=%d\n",
                (int)getpid(), g_opaque_policy ? 1 : 0, g_vsync_override, g_max_fps, g_spatial_factor,
                (double)g_sharpness, g_sharpen_only ? 1 : 0);
        fflush(stderr);
    }
}
