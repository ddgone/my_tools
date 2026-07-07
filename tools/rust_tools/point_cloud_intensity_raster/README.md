# point_cloud_intensity_raster

LAS/LAZ 强度图生成工具。

该工具面向已经成型的 LAS/LAZ 点云文件，只负责根据点云强度生成栅格 PNG 预览图与常见 GIS 侧车文件，不做抽稀、白犀牛任务包解析或 UTM 汇总。

## 当前能力

- 支持单个 `.las` / `.laz` 文件输入
- 支持目录递归批量输入
- 支持并发处理多个点云文件
- 支持输出：
  - `png`
  - `pgw`
  - `prj`
  - `aux.xml`
  - `vrt`

## 主要参数

- `--input <PATH>`：输入文件或目录
- `--output <DIR>`：输出目录
- `--resolution <METER>`：强度图像素分辨率
- `--threads <N>`：并发线程数
- `--force`：覆盖已有输出

## 输出规则

- 单文件模式：输出到指定目录
- 目录模式：递归扫描输入目录中的 `.las/.laz`，并在输出目录中保留相对目录结构
- 输出文件名会附带强度图后缀，例如：
  - `sample.laz -> sample_intensity.png`

## 常用示例

```bash
point_cloud_intensity_raster --input D:\clouds --output D:\raster_out --resolution 0.5 --threads 8
```

```bash
point_cloud_intensity_raster --input D:\clouds\a.las --output D:\raster_out --resolution 0.2
```

```bash
point_cloud_intensity_raster --input D:\clouds --output D:\raster_out --resolution 1.0 --force
```
