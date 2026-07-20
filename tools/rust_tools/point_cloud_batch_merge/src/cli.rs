use clap::{Parser, ValueEnum};
use std::path::PathBuf;

#[derive(Debug, Parser)]
#[command(
    name = "point_cloud_batch_merge",
    about = "将单帧PCD点云tar.gz包批量合并为LAS/LAZ文件，使用POS轨迹数据定位"
)]
pub struct Cli {
    /// 单帧点云目录（包含 .tar.gz 压缩包）
    #[arg(short, long, value_name = "DIR")]
    pub input: PathBuf,

    /// POS轨迹目录（包含 JSON 格式轨迹文件）
    #[arg(short = 'p', long = "pos-dir", value_name = "DIR")]
    pub pos_dir: PathBuf,

    /// 输出目录
    #[arg(short, long, value_name = "DIR")]
    pub output: PathBuf,

    /// 输出格式
    #[arg(short = 'f', long = "output-format", value_enum, default_value_t = OutputFormat::Laz)]
    pub format: OutputFormat,

    /// 并行处理线程数
    #[arg(short, long, value_name = "N", default_value_t = 4)]
    pub threads: usize,

    /// 翻转Z轴（默认开启，使用 --no-flip-z 关闭）
    #[arg(long, default_value_t = true)]
    pub flip_z: bool,
}

#[derive(Copy, Clone, Debug, Eq, PartialEq, ValueEnum)]
pub enum OutputFormat {
    Laz,
    Las,
}

impl OutputFormat {
    pub fn extension(self) -> &'static str {
        match self {
            OutputFormat::Laz => "laz",
            OutputFormat::Las => "las",
        }
    }
}
