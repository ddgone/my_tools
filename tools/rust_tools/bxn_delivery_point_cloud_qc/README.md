# bxn_delivery_point_cloud_qc

白犀牛交付点云质检预处理工具。

该工具面向白犀牛交付任务包批处理场景，扫描输入根目录下一层符合命名规范的数据包目录，读取每包中的 `process_result_0/deskew_cloud`、`opti_pose_enu.txt` 与 `utm.txt`，输出强度图、UTM 汇总、单包日志、状态文件，以及可选的抽稀点云文件。

## 当前能力

- 支持目录模式任务包
- 支持直接读取 `process_result_0/deskew_cloud`
- 支持自动解压 `process_result_0.tar.gz`
- 支持统一 `origin.txt`
- 支持台账跳过
- 支持点云输出三态：
  - `laz`
  - `las`
  - `none`

## 输出内容

每个数据包会产出以下结果：

- 抽稀点云文件：`LAZ` 或 `LAS`，也可关闭
- 强度图：`intensity.png`
- 侧车文件：世界文件、`prj`、`aux.xml`、`vrt`
- 收集后的 `utm.txt`
- 单包日志
- 状态文件
- 批次台账 `run_ledger.json`

## 主要参数

- `--input <DIR>`：输入根目录，扫描其下一层数据包目录
- `--output <DIR>`：输出根目录
- `--origin <FILE>`：可选，统一 `origin.txt`
- `--threads <N>`：单包内部处理线程数，默认 `4`
- `--voxel-size <METER>`：体素抽稀边长，默认 `0.2`
- `--ledger <FILE>`：可选，指定已有台账文件做重试
- `--output-format <laz|las|none>`：点云输出格式，默认 `laz`

## 常用示例

标准运行：

```bash
bxn_delivery_point_cloud_qc --input D:\jobs --output D:\qc_out --origin D:\origin.txt --threads 8 --voxel-size 0.2
```

输出 LAS：

```bash
bxn_delivery_point_cloud_qc --input D:\jobs --output D:\qc_out --threads 8 --output-format las
```

只生成强度图与侧车文件，不输出点云文件：

```bash
bxn_delivery_point_cloud_qc --input D:\jobs --output D:\qc_out --threads 8 --output-format none
```

不抽稀：

```bash
bxn_delivery_point_cloud_qc --input D:\jobs --output D:\qc_out --voxel-size 0
```

## 输出目录说明

点云输出目录会根据体素配置和输出格式分开落盘，例如：

- `voxel_0.2_laz/<dataset>/output_0.2.laz`
- `voxel_0.2_las/<dataset>/output_0.2.las`
- `voxel_0.2_none/<dataset>/output_0.2.done.json`

当 `--output-format none` 时，不会生成点云文件，但仍会生成状态文件、强度图、UTM 汇总和日志。

## 台账兼容

历史台账中的 `output_laz` 字段仍可读取，用于兼容旧版本结果。
新版本统一记录为通用的点云输出路径与输出格式。
