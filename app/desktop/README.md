# 火蜥蜴工具箱 Desktop

这是当前桌面宿主子模块，不再是默认的 Wails Vue 模板示例页。

## 正确运行方式

开发模式：

```powershell
cd app/desktop
& "$env:USERPROFILE\go\bin\wails.exe" dev
```

生产构建：

```powershell
cd app/desktop
& "$env:USERPROFILE\go\bin\wails.exe" build -clean
```

正式产物位置：

```text
app/desktop/build/bin/fire-salamander-desktop.exe
```

## 重要说明

- 不要把 `app/desktop` 根目录下的 `.exe` 当成正式桌面产物。
- 如果你在该目录执行 `go build`，Go 会生成一个缺少 Wails 正确构建标签的误产物。
- 当前桌面应用唯一正确的构建方式是 `wails build`。
