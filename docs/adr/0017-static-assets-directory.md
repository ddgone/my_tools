# 在安装镜像目录下建立 assets/ 静态资源目录

我们决定在安装镜像目录 `build/image/host/` 下建立 `assets/` 子目录，用于存放不需要嵌入 exe 的只读静态资源。这些资源（如启动画面图片、字体文件、默认图标）的源文件放在 `app/assets/`（受版本控制），由构建脚本在 `wails build` 完成后复制到 `build/image/host/assets/`。Go 端在运行时通过 `executable` 相对路径读取这些文件，不使用 `go:embed`。这样做的原因是：部分资源（如图片）嵌入 exe 会增大二进制体积，且更新资源时不需要重新编译。`build/` 目录整体已在 `.gitignore` 中，`assets/` 子目录跟随安装镜像目录一起被构建脚本整目录清理和重建，不会残留过时文件。
