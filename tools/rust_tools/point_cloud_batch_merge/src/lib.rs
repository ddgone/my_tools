//! 将单帧PCD点云tar.gz包批量合并为LAS/LAZ文件，使用POS轨迹数据进行定位。
//!
//! 每个 tar.gz 包输出一个 LAZ/LAS 文件，
//! PCD 时间戳与 POS JSON 时间戳自动匹配，
//! 使用方位角旋转 + WGS84 → UTM 投影。

mod cli;
mod merge;
mod pcd;
mod pos;
mod utm;

use anyhow::Result;
use clap::Parser;

pub fn run(args: &[String]) -> Result<()> {
    let cli = cli::Cli::try_parse_from(
        std::iter::once("point_cloud_batch_merge").chain(args.iter().map(String::as_str)),
    )?;
    merge::run(&cli)
}
