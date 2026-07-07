# point_cloud_voxel_downsample

LAS/LAZ 点云体素抽稀工具。

该工具面向已经成型的 LAS/LAZ 点云文件，只做体素抽稀，不做白犀牛任务包解析、ENU 转换、强度图或 UTM 汇总。

## 当前能力

- 支持单个 `.las` / `.laz` 文件输入
- 支持目录递归批量输入
- 支持并发处理多个点云文件
- 支持 `first / center` 两种代表点策略
- 支持输出格式：
  - 保持输入格式
  - 强制输出 `laz`
  - 强制输出 `las`

## 主要参数

- `--input <PATH>`：输入文件或目录
- `--output <DIR>`：输出目录
- `--voxel-size <METER>`：体素大小，`0` 表示不抽稀
- `--representative <first|center>`：体素代表点策略
- `--output-format <preserve|laz|las>`：输出点云格式
- `--threads <N>`：并发线程数
- `--force`：覆盖已有输出

## 输出规则

- 单文件模式：输出到指定目录
- 目录模式：递归扫描输入目录中的 `.las/.laz`，并在输出目录中保留相对目录结构
- 默认文件名会附带抽稀后缀，例如：
  - `sample.laz -> sample_voxel_0.2.laz`
  - `sample.las -> sample_voxel_1.las`

## 常用示例

```bash
point_cloud_voxel_downsample --input D:\clouds --output D:\voxel_out --voxel-size 0.2 --threads 8
```

```bash
point_cloud_voxel_downsample --input D:\clouds\a.laz --output D:\voxel_out --voxel-size 0.5 --output-format las
```

```bash
point_cloud_voxel_downsample --input D:\clouds --output D:\voxel_out --voxel-size 0 --force
```
