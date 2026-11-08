// SPDX-License-Identifier: GPL-3.0-only
#include <Windows.h>
#include <activation.h>
#include <new>
#include <string_view>
#include <winstring.h>

extern "C" __declspec(dllexport) int NativeFixtureValue() { return 42; }

namespace {
// Stands in for a Windows App SDK class served by Microsoft.WindowsAppRuntime.dll.
constexpr std::wstring_view test_class = L"Microsoft.LeviLauncher.Test";

struct TestFactory final : IActivationFactory {
  LONG references = 1;
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid, void **object) override {
    if (!object)
      return E_POINTER;
    if (iid == __uuidof(IUnknown) || iid == __uuidof(IInspectable) ||
        iid == __uuidof(IActivationFactory)) {
      *object = static_cast<IActivationFactory *>(this);
      AddRef();
      return S_OK;
    }
    *object = nullptr;
    return E_NOINTERFACE;
  }
  ULONG STDMETHODCALLTYPE AddRef() override { return InterlockedIncrement(&references); }
  ULONG STDMETHODCALLTYPE Release() override {
    const auto remaining = InterlockedDecrement(&references);
    if (!remaining)
      delete this;
    return remaining;
  }
  HRESULT STDMETHODCALLTYPE GetIids(ULONG *count, IID **iids) override {
    *count = 0;
    *iids = nullptr;
    return S_OK;
  }
  HRESULT STDMETHODCALLTYPE GetRuntimeClassName(HSTRING *name) override {
    return WindowsCreateString(test_class.data(), static_cast<UINT32>(test_class.size()), name);
  }
  HRESULT STDMETHODCALLTYPE GetTrustLevel(TrustLevel *level) override {
    *level = BaseTrust;
    return S_OK;
  }
  HRESULT STDMETHODCALLTYPE ActivateInstance(IInspectable **instance) override {
    *instance = nullptr;
    return E_NOTIMPL;
  }
};
} // namespace

extern "C" __declspec(dllexport) HRESULT WINAPI DllGetActivationFactory(HSTRING class_id,
                                                                         IActivationFactory **factory) {
  if (!factory)
    return E_POINTER;
  *factory = nullptr;
  UINT32 length = 0;
  const wchar_t *name = WindowsGetStringRawBuffer(class_id, &length);
  if (std::wstring_view(name, length) != test_class)
    return CLASS_E_CLASSNOTAVAILABLE;
  *factory = new (std::nothrow) TestFactory;
  return *factory ? S_OK : E_OUTOFMEMORY;
}
