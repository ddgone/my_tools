use anyhow::{Context, Result, bail};
use std::fs;
use std::path::{Path, PathBuf};
use std::time::UNIX_EPOCH;

use crate::legacy::PointCloudOutputFormat;

use super::discovery::required_outputs;
use super::paths::{ensure_parent_dir, format_float, system_time_to_epoch_ms, unix_now};
use super::types::{
    BatchTask, DEFAULT_LEDGER_NAME, FileFingerprint, LedgerFile, LedgerStatus, PackageFingerprint,
};

pub(crate) fn should_skip_by_ledger(
    task: &BatchTask,
    current_fingerprint: Option<&PackageFingerprint>,
    ledger: &LedgerFile,
    output_format: PointCloudOutputFormat,
) -> bool {
    let Some(entry) = ledger.packages.get(&task.dataset_name) else {
        return false;
    };
    if entry.status != LedgerStatus::Success {
        return false;
    }
    let Some(current_fingerprint) = current_fingerprint else {
        return false;
    };
    let Some(previous_fingerprint) = entry.fingerprint.as_ref() else {
        return false;
    };
    if previous_fingerprint != current_fingerprint {
        return false;
    }
    required_outputs(
        task,
        ledger.origin_path.as_deref().map(Path::new),
        output_format,
    )
    .into_iter()
    .all(|path| path.is_file())
        || {
            let Some(output_point_cloud) = entry.output_point_cloud.as_ref() else {
                return !output_format.writes_point_cloud()
                    && PathBuf::from(&entry.intensity_png).is_file();
            };
            let output_laz = PathBuf::from(output_point_cloud);
            let intensity_png = PathBuf::from(&entry.intensity_png);
            output_laz.is_file()
                && intensity_png.is_file()
                && (!output_format.writes_point_cloud() || entry.output_format == output_format)
        }
}

pub(crate) fn load_or_init_ledger(
    ledger_path: &Path,
    input_root: &Path,
    output_root: &Path,
    origin: Option<&Path>,
) -> Result<LedgerFile> {
    if ledger_path.is_file() {
        let content = fs::read_to_string(ledger_path)
            .with_context(|| format!("无法读取台账文件: {}", ledger_path.display()))?;
        let ledger: LedgerFile = serde_json::from_str(&content)
            .with_context(|| format!("台账 JSON 无法解析: {}", ledger_path.display()))?;
        return Ok(ledger);
    }
    Ok(LedgerFile {
        version: 1,
        created_at_epoch_s: unix_now(),
        updated_at_epoch_s: unix_now(),
        input_root: input_root.display().to_string(),
        output_root: output_root.display().to_string(),
        origin_path: origin.map(|path| path.display().to_string()),
        packages: Default::default(),
    })
}

pub(crate) fn flush_ledger(path: &Path, ledger: &mut LedgerFile) -> Result<()> {
    ledger.updated_at_epoch_s = unix_now();
    ensure_parent_dir(path)?;
    let payload = serde_json::to_string_pretty(ledger).context("生成台账 JSON 失败")?;
    fs::write(path, payload).with_context(|| format!("无法写出台账: {}", path.display()))
}

pub(crate) fn compute_package_fingerprint(
    task: &BatchTask,
    origin: Option<&Path>,
    voxel_size: f64,
    output_format: PointCloudOutputFormat,
) -> Result<PackageFingerprint> {
    let voxel_setting = Some(voxel_setting_signature(voxel_size));
    let output_format = Some(output_format.display_name().to_string());
    if let Some(archive_path) = &task.source_archive_path
        && archive_path.is_file()
    {
        return Ok(PackageFingerprint {
            dataset_name: task.dataset_name.clone(),
            source_kind: Some("tar.gz".to_string()),
            archive_file: Some(file_fingerprint(archive_path)?),
            enu_file: FileFingerprint {
                size: 0,
                modified_epoch_ms: 0,
            },
            pcd_file_count: 0,
            pcd_total_size: 0,
            pcd_latest_modified_epoch_ms: 0,
            origin_file: origin.map(file_fingerprint).transpose()?,
            voxel_setting,
            output_format,
        });
    }

    let enu_file = file_fingerprint(&task.enu_path)?;
    let mut pcd_file_count = 0usize;
    let mut pcd_total_size = 0u64;
    let mut pcd_latest_modified_epoch_ms = 0u64;
    for entry in fs::read_dir(&task.pcd_dir)
        .with_context(|| format!("无法读取 PCD 目录: {}", task.pcd_dir.display()))?
    {
        let entry = entry?;
        if !entry.file_type()?.is_file() {
            continue;
        }
        let path = entry.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("pcd") {
            continue;
        }
        let metadata = entry.metadata()?;
        pcd_file_count += 1;
        pcd_total_size = pcd_total_size.saturating_add(metadata.len());
        pcd_latest_modified_epoch_ms = pcd_latest_modified_epoch_ms.max(system_time_to_epoch_ms(
            metadata.modified().unwrap_or(UNIX_EPOCH),
        ));
    }
    Ok(PackageFingerprint {
        dataset_name: task.dataset_name.clone(),
        source_kind: Some("directory".to_string()),
        archive_file: None,
        enu_file,
        pcd_file_count,
        pcd_total_size,
        pcd_latest_modified_epoch_ms,
        origin_file: origin.map(file_fingerprint).transpose()?,
        voxel_setting,
        output_format,
    })
}

fn file_fingerprint(path: &Path) -> Result<FileFingerprint> {
    let metadata =
        fs::metadata(path).with_context(|| format!("无法读取文件信息: {}", path.display()))?;
    Ok(FileFingerprint {
        size: metadata.len(),
        modified_epoch_ms: system_time_to_epoch_ms(metadata.modified().unwrap_or(UNIX_EPOCH)),
    })
}

fn voxel_setting_signature(voxel_size: f64) -> String {
    if voxel_size <= 0.0 {
        "disabled".to_string()
    } else {
        format!("voxel={}", format_float(voxel_size))
    }
}

pub(crate) fn prepare_output_root(
    output_root: &Path,
    ledger_path: &Path,
    allow_existing_outputs: bool,
) -> Result<()> {
    if output_root.exists() {
        if !output_root.is_dir() {
            bail!("--output 不是目录: {}", output_root.display());
        }
        if !allow_existing_outputs {
            let mut entries = fs::read_dir(output_root)
                .with_context(|| format!("无法读取输出目录: {}", output_root.display()))?;
            if entries.next().is_some() {
                bail!(
                    "输出目录已存在且非空，如需重试请传 --ledger 指向已有台账: {}",
                    output_root.display()
                );
            }
        }
    } else {
        fs::create_dir_all(output_root)
            .with_context(|| format!("无法创建输出目录: {}", output_root.display()))?;
    }

    if allow_existing_outputs
        && ledger_path != output_root.join(DEFAULT_LEDGER_NAME)
        && !ledger_path.exists()
    {
        bail!("重试模式需要已有台账文件: {}", ledger_path.display());
    }
    Ok(())
}
