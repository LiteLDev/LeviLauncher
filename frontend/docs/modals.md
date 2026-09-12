# 弹窗接口

应用弹窗统一使用 `UnifiedModal`；删除确认使用 `DeleteConfirmModal`。
图片预览等需要自定义标题和内容结构的场景可以组合 `BaseModal`、
`BaseModalHeader`、`BaseModalBody` 和 `BaseModalFooter`。三者共享
`BaseModalProps` 中的尺寸与状态类型，视觉参数只在 `BaseModal.tsx` 定义。

## 尺寸

| `size` | 最大宽度 | 用途 |
| --- | --- | --- |
| `compact` | 560px | 简短确认 |
| `standard`（默认） | 680px | 提示、删除确认、普通表单 |
| `wide` | 880px | 实例选择、更新日志、多列表单 |
| `detail` | 1120px | 图片预览、复杂编辑 |
| `full` | 全屏 | 需要占满窗口的内容 |

宽度作用于 `Modal.Dialog`，外层 `Modal.Container` 只负责定位和窗口边距。
不要通过页面 `className` 单独设置宽度、圆角和重复的外壳内边距。
内容自然决定高度；默认 `scrollBehavior="inside"` 保留标题和操作区，正文超出时滚动。
`outside` 仅用于明确需要整个弹窗滚动的场景。

## 普通弹窗

```tsx
<UnifiedModal
  isOpen={isOpen}
  onOpenChange={setIsOpen}
  type="primary"
  title={t("example.title")}
  size="wide"
  isDismissable
  isPending={saving}
  showCancelButton
  onCancel={() => setIsOpen(false)}
  onConfirm={save}
  confirmText={t("common.confirm")}
  confirmButtonProps={{ isDisabled: !isValid }}
>
  {content}
</UnifiedModal>
```

- `type` 决定默认图标和状态颜色，支持 `primary / info / success / warning / error`。
  普通确认、完成和关闭操作统一使用主题主色；删除、卸载和重置等危险操作显式传入 `confirmButtonProps={{ variant: "danger" }}`。
- `onConfirm` 由业务管理成功关闭与失败展示，不会自动关闭；省略它时不会凭空生成操作区。
- `showCancelButton` 显示取消按钮；未提供 `onCancel` 时通过 `onOpenChange(false)` 关闭。
- `confirmText / cancelText` 覆盖国际化默认文案。
- `confirmButtonProps / cancelButtonProps` 统一传递按钮禁用状态、变体和扩展样式。
- `isPending` 控制主按钮加载状态，同时禁用默认操作按钮、遮罩关闭与 Escape。取消按钮保持可见，避免操作区跳动。
  既有 `confirmButtonProps.isPending` 仍作为后备值，新增调用优先使用顶层 `isPending`。
- `isDismissable` 控制遮罩点击关闭，默认 `false`；`isKeyboardDismissDisabled` 单独控制 Escape，默认 `false`，保留现有键盘行为。
- 所有弹窗均无右上角关闭按钮，也不再提供 `hideCloseButton` 参数；关闭、取消和返回等操作放在底部。
- `footer` 是复杂操作区的扩展入口（多操作、进度控制等），替换默认按钮区；
  自定义按钮的加载、禁用与事件由业务负责。普通确认操作优先使用标准按钮参数。

删除弹窗补充 `description / itemName / itemNames / warning / error`。
其 `onConfirm` 返回 `false` 会保留弹窗，抛错会显示错误，成功则关闭；
异步操作需传入 `isPending`，沿用统一交互规则。

## 内容与排版

`ModalPrimitives.tsx` 提供共享内容组件，页面只负责业务数据和交互。

| 组件 | 用途 |
| --- | --- |
| `ModalDescription` | 左对齐说明，14px 字号、24px 行高、常规字重 |
| `ModalPanel` | 文件名、路径、选择结果等中性信息卡片 |
| `ModalNotice` | 信息、风险、错误提示，统一图标、边框与背景；动态错误传 `role="alert"` |
| `ModalDetails` | 使用 `dl / dt / dd` 的标签和值，桌面两列、小窗口单列；长路径用 `fullWidth` |
| `ModalProgress` | 说明 → 进度条 → 进度明细 → 当前目标；未知进度不展示虚假百分比 |
| `ModalAction` | 自定义底部按钮，统一圆角、字号、最小高度和间距 |

标题为 20px 半粗体，图标区域为 40px；标题与正文左右对齐，段落间距为 16px。
整个弹窗使用同一背景，底部操作区不设置独立底色或分隔线，仅通过间距区分内容与操作。
取消在左、主要操作在右；长按钮文案允许换行。
使用 `footer` 时也必须使用 `ModalAction`，避免重新引入页面独有的按钮阴影、圆角和大小。

## 页面覆盖

| 页面／组件 | 统一内容 |
| --- | --- |
| 启动页 | 依赖缺失、安装进度、启动状态、注册结果、快捷方式提示、启动错误 |
| 下载与安装页 | 镜像选择、安装进度、安装错误、下载包删除、预发行风险确认 |
| 实例设置 | 备份／恢复表单与进度、结果摘要、注册与卸载、加载器选择、安装确认、未保存提示 |
| 内容、资源包、行为包、皮肤包、世界列表 | 相同的转移进度、目标选择、覆盖确认、删除确认与结果列表 |
| CurseForge | 下载、选择实例／玩家、导入、成功、失败、覆盖确认 |
| LIP 与软件包详情 | 开发者提示、依赖提示、实例选择与版本摘要 |
| 模组页 | 导入进度、DLL 表单、详情、编辑、覆盖确认、批量操作 |
| 设置与引导页 | 进程列表、实验性风险提示、安装进度、未保存、重置与路径错误 |
| 更新、协议与隐私组件 | 相同标题和操作区、版本摘要、长文排版 |
| 截图预览、世界编辑器与任务控制台 | 共享外壳和按钮，保留图片工具栏、保存分支与日志滚动结构 |

代码层面所有弹窗均经过统一组件迁移；浏览器回归重点验证代表性页面和交互状态，
不把模拟 Wails 后端测试视作所有原生安装、恢复与卸载路径的实机验收。
