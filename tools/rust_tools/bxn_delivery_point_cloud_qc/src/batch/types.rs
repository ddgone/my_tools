use crate::legacy::{
    MappingMode, PcdProcessReport, PivotMode, PointCloudOutputFormat, RepresentativeMode,
};
use anyhow::{Context, Result};
use clap::Parser;
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::fs::{File, OpenOptions};
use std::io::{BufWriter, Write};
use std::path::{Path, PathBuf};

use super::paths::ensure_parent_dir;

pub(crate) const DEFAULT_VOXEL_SIZE: f64 = 0.2;
pub(crate) const DEFAULT_INTENSITY_RESOLUTION: f64 = 0.5;
pub(crate) const DEFAULT_LEDGER_NAME: &str = "run_ledger.json";

#[derive(Debug, Parser)]
#[command(
    author,
    version,
    about = "批量扫描 0 阶段数据包中的 process_result_0/deskew_cloud 与 opti_pose_enu.txt，输出强度图，并可选输出最终抽稀 LAZ/LAS"
)]
pub struct BatchCli {
    #[arg(short, long, value_name = "DIR")]
    pub(crate) input: PathBuf,
    #[arg(short, long, value_name = "DIR")]
    pub(crate) output: PathBuf,
    #[arg(long, value_name = "FILE")]
    pub(crate) origin: Option<PathBuf>,
    #[arg(long, value_name = "N", default_value_t = 4)]
    pub(crate) threads: usize,
    #[arg(
        long,
        value_name = "METER",
        default_value_t = DEFAULT_VOXEL_SIZE,
        help = "体素抽稀边长，单位米；0 表示不抽稀，0.1 表示 10cm，默认 0.2"
    )]
    pub(crate) voxel_size: f64,
    #[arg(long, value_name = "FILE")]
    pub(crate) ledger: Option<PathBuf>,
    #[arg(
        long,
        value_enum,
        default_value_t = PointCloudOutputFormat::Laz,
        help = "点云输出格式：laz / las / none；none 表示不输出点云文件，仅输出强度图、侧车文件、UTM 收集结果和状态文件"
    )]
    pub(crate) output_format: PointCloudOutputFormat,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum TaskInputKind {
    Directory,
    TarGzArchive,
}

#[derive(Debug, Clone)]
pub(crate) struct BatchTask {
    pub(crate) dataset_name: String,
    pub(crate) dataset_dir: PathBuf,
    pub(crate) input_kind: TaskInputKind,
    pub(crate) source_archive_path: Option<PathBuf>,
    pub(crate) cleanup_dir: Option<PathBuf>,
    pub(crate) process_root: PathBuf,
    pub(crate) pcd_dir: PathBuf,
    pub(crate) enu_path: PathBuf,
    pub(crate) utm_path: PathBuf,
    pub(crate) output_point_cloud: Option<PathBuf>,
    pub(crate) output_format: PointCloudOutputFormat,
    pub(crate) intensity_png: PathBuf,
    pub(crate) utm_collected_path: PathBuf,
    pub(crate) package_log_path: PathBuf,
    pub(crate) status_path: PathBuf,
}

impl BatchTask {
    pub(crate) fn label(&self) -> String {
        self.dataset_name.clone()
    }
}

#[derive(Debug, Serialize)]
pub(crate) struct StatusFile {
    pub(crate) completed_at_epoch_s: u64,
    pub(crate) dataset_name: String,
    pub(crate) dataset_dir: String,
    pub(crate) source_archive_path: Option<String>,
    pub(crate) pcd_dir: String,
    pub(crate) enu_path: String,
    pub(crate) utm_path: String,
    pub(crate) output_point_cloud: Option<String>,
    pub(crate) output_format: PointCloudOutputFormat,
    pub(crate) intensity_png: String,
    pub(crate) utm_collected_path: String,
    pub(crate) package_log: String,
    pub(crate) origin_path: Option<String>,
    pub(crate) voxel_size: f64,
    pub(crate) intensity_resolution: f64,
    pub(crate) representative: &'static str,
    pub(crate) threads: usize,
    pub(crate) pivot: &'static str,
    pub(crate) mapping: &'static str,
    pub(crate) output_enabled: bool,
    pub(crate) report: PcdProcessReport,
    pub(crate) required_outputs: Vec<String>,
}

#[derive(Debug, Serialize, Deserialize, Clone, Copy, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub(crate) enum LedgerStatus {
    Success,
    Skipped,
    Failed,
}

#[derive(Debug, Serialize, Deserialize, Clone, PartialEq, Eq)]
pub(crate) struct FileFingerprint {
    pub(crate) size: u64,
    pub(crate) modified_epoch_ms: u64,
}

#[derive(Debug, Serialize, Deserialize, Clone, PartialEq, Eq)]
pub(crate) struct PackageFingerprint {
    pub(crate) dataset_name: String,
    pub(crate) source_kind: Option<String>,
    pub(crate) archive_file: Option<FileFingerprint>,
    pub(crate) enu_file: FileFingerprint,
    pub(crate) pcd_file_count: usize,
    pub(crate) pcd_total_size: u64,
    pub(crate) pcd_latest_modified_epoch_ms: u64,
    pub(crate) origin_file: Option<FileFingerprint>,
    #[serde(default)]
    pub(crate) voxel_setting: Option<String>,
    #[serde(default)]
    pub(crate) output_format: Option<String>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub(crate) struct LedgerPackageEntry {
    pub(crate) status: LedgerStatus,
    pub(crate) last_run_epoch_s: u64,
    pub(crate) message: String,
    pub(crate) fingerprint: Option<PackageFingerprint>,
    #[serde(default, alias = "output_laz")]
    pub(crate) output_point_cloud: Option<String>,
    #[serde(default)]
    pub(crate) output_format: PointCloudOutputFormat,
    pub(crate) intensity_png: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub(crate) struct LedgerFile {
    pub(crate) version: u32,
    pub(crate) created_at_epoch_s: u64,
    pub(crate) updated_at_epoch_s: u64,
    pub(crate) input_root: String,
    pub(crate) output_root: String,
    pub(crate) origin_path: Option<String>,
    pub(crate) packages: BTreeMap<String, LedgerPackageEntry>,
}

pub(crate) struct BatchLogger {
    file: BufWriter<File>,
}

impl BatchLogger {
    pub(crate) fn new(path: &Path) -> Result<Self> {
        ensure_parent_dir(path)?;
        let file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(path)
            .with_context(|| format!("无法打开批次日志: {}", path.display()))?;
        Ok(Self {
            file: BufWriter::new(file),
        })
    }

    pub(crate) fn log(&mut self, level: &str, message: impl AsRef<str>) {
        let line = format!("[{}] {}", level, message.as_ref());
        eprintln!("{line}");
        let _ = writeln!(self.file, "{line}");
        let _ = self.file.flush();
    }
}

#[cfg(test)]
mod tests {
    use super::{LedgerPackageEntry, PointCloudOutputFormat};

    #[test]
    fn should_deserialize_legacy_output_laz_field() {
        let entry: LedgerPackageEntry = serde_json::from_str(
            r#"{
                "status":"success",
                "last_run_epoch_s":123,
                "message":"ok",
                "fingerprint":null,
                "output_laz":"D:\\out\\sample.laz",
                "intensity_png":"D:\\out\\intensity.png"
            }"#,
        )
        .expect("legacy ledger entry should deserialize");

        assert_eq!(
            entry.output_point_cloud.as_deref(),
            Some(r"D:\out\sample.laz")
        );
        assert_eq!(entry.output_format, PointCloudOutputFormat::Laz);
    }

    #[test]
    fn should_report_output_format_metadata() {
        assert_eq!(PointCloudOutputFormat::Laz.file_extension(), Some("laz"));
        assert_eq!(PointCloudOutputFormat::Las.file_extension(), Some("las"));
        assert_eq!(PointCloudOutputFormat::None.file_extension(), None);
        assert!(!PointCloudOutputFormat::None.writes_point_cloud());
    }
}

pub(crate) fn representative_name(mode: RepresentativeMode) -> &'static str {
    match mode {
        RepresentativeMode::First => "first",
        RepresentativeMode::Center => "center",
    }
}

pub(crate) fn pivot_name(mode: PivotMode) -> &'static str {
    match mode {
        PivotMode::Centroid => "centroid",
        PivotMode::BboxCenter => "bbox-center",
        PivotMode::Zero => "zero",
    }
}

pub(crate) fn mapping_name(mode: MappingMode) -> &'static str {
    match mode {
        MappingMode::Enu => "enu",
        MappingMode::Flat => "flat",
    }
}
