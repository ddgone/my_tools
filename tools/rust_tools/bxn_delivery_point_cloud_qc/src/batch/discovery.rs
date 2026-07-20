use crate::legacy::{
    MappingMode, PcdProcessReport, PivotMode, PointCloudOutputFormat, RepresentativeMode,
};
use anyhow::{Context, Result, anyhow, bail};
use flate2::read::GzDecoder;
use std::ffi::OsStr;
use std::fs;
use std::fs::File;
use std::io::BufReader;
use std::path::{Path, PathBuf};
#[cfg(windows)]
use std::process::{Command, Stdio};
use std::thread;
use std::time::Duration;
use tar::Archive;

use super::paths::{
    aux_xml_path, ensure_parent_dir, format_float, is_dataset_name, paths_equivalent, sidecar_path,
    unix_now, vrt_path, world_file_path,
};
use super::types::{
    BatchTask, StatusFile, TaskInputKind, mapping_name, pivot_name, representative_name,
};

const PROCESS_ROOT_DIR: &str = "process_result_0";
const PROCESS_ARCHIVE_NAME: &str = "process_result_0.tar.gz";
const TEMP_WORK_ROOT_DIR: &str = ".bxn_batch_tmp";
const CLEANUP_RETRY_ATTEMPTS: usize = 8;
const CLEANUP_RETRY_DELAY_MS: u64 = 250;

pub(crate) fn discover_tasks(
    input_root: &Path,
    output_root: &Path,
    voxel_size: f64,
    output_format: PointCloudOutputFormat,
) -> Result<Vec<BatchTask>> {
    let mut directories = Vec::new();
    for entry in fs::read_dir(input_root)
        .with_context(|| format!("无法读取输入目录: {}", input_root.display()))?
    {
        let entry = entry?;
        if !entry.file_type()?.is_dir() {
            continue;
        }
        let path = entry.path();
        if paths_equivalent(&path, output_root) {
            continue;
        }
        let Some(name) = path.file_name().and_then(OsStr::to_str) else {
            continue;
        };
        if !is_dataset_name(name) {
            continue;
        }
        directories.push(path);
    }
    directories.sort();
    Ok(directories
        .into_iter()
        .map(|directory| build_task(directory, output_root, voxel_size, output_format))
        .collect())
}

fn build_task(
    directory: PathBuf,
    output_root: &Path,
    voxel_size: f64,
    output_format: PointCloudOutputFormat,
) -> BatchTask {
    let dataset_name = directory
        .file_name()
        .and_then(OsStr::to_str)
        .unwrap_or("unknown")
        .to_string();
    let source_archive_path = directory.join(PROCESS_ARCHIVE_NAME);
    let input_kind = if source_archive_path.is_file() {
        TaskInputKind::TarGzArchive
    } else {
        TaskInputKind::Directory
    };
    let (cleanup_dir, process_root) = match input_kind {
        TaskInputKind::Directory => (None, directory.join(PROCESS_ROOT_DIR)),
        TaskInputKind::TarGzArchive => {
            let cleanup_dir = output_root.join(TEMP_WORK_ROOT_DIR).join(&dataset_name);
            (
                Some(cleanup_dir.clone()),
                cleanup_dir.join(PROCESS_ROOT_DIR),
            )
        }
    };
    let voxel_tag = voxel_output_tag(voxel_size);
    let output_stem = if voxel_size > 0.0 {
        format!("output_{voxel_tag}")
    } else {
        "output_full".to_string()
    };
    let voxel_dir = output_root
        .join(format!("voxel_{voxel_tag}_{}", output_format.dir_suffix()))
        .join(&dataset_name);
    let intensity_dir = output_root.join("intensity_png").join(&dataset_name);
    let logs_dir = output_root.join("logs").join(&dataset_name);
    let utm_col_dir = output_root.join("utm_col");
    let output_point_cloud = output_format
        .file_extension()
        .map(|ext| voxel_dir.join(format!("{output_stem}.{ext}")));
    let utm_collected_path = utm_col_dir.join(format!("{dataset_name}.utm.txt"));
    BatchTask {
        dataset_name,
        dataset_dir: directory,
        input_kind,
        source_archive_path: source_archive_path.is_file().then_some(source_archive_path),
        cleanup_dir,
        process_root: process_root.clone(),
        pcd_dir: process_root.join("deskew_cloud"),
        enu_path: process_root.join("opti_pose_enu.txt"),
        utm_path: process_root.join("utm.txt"),
        output_format,
        intensity_png: intensity_dir.join("intensity.png"),
        utm_collected_path,
        package_log_path: logs_dir.join(format!("{output_stem}.log")),
        status_path: voxel_dir.join(format!("{output_stem}.done.json")),
        output_point_cloud,
    }
}

pub(crate) fn validate_task_layout(task: &BatchTask) -> Result<()> {
    if task.input_kind == TaskInputKind::TarGzArchive {
        let Some(archive_path) = task.source_archive_path.as_ref() else {
            bail!(
                "缺少任务压缩包: {}",
                task.dataset_dir.join(PROCESS_ARCHIVE_NAME).display()
            );
        };
        if !archive_path.is_file() {
            bail!("任务压缩包不是文件: {}", archive_path.display());
        }
    }
    if !task.pcd_dir.exists() {
        bail!("缺少 deskew_cloud 目录: {}", task.pcd_dir.display());
    }
    if !task.pcd_dir.is_dir() {
        bail!("deskew_cloud 不是目录: {}", task.pcd_dir.display());
    }
    if !task.enu_path.exists() {
        bail!("缺少 ENU 文件: {}", task.enu_path.display());
    }
    if !task.enu_path.is_file() {
        bail!("ENU 路径不是文件: {}", task.enu_path.display());
    }
    if !task.utm_path.exists() {
        bail!("缺少 UTM 文件: {}", task.utm_path.display());
    }
    if !task.utm_path.is_file() {
        bail!("UTM 路径不是文件: {}", task.utm_path.display());
    }
    Ok(())
}

pub(crate) fn prepare_task_input(task: &BatchTask) -> Result<bool> {
    if task.input_kind != TaskInputKind::TarGzArchive {
        return Ok(false);
    }
    if extracted_layout_ready(task) {
        return Ok(false);
    }

    let archive_path = task
        .source_archive_path
        .as_ref()
        .context("压缩包任务缺少 source_archive_path")?;
    let cleanup_dir = task
        .cleanup_dir
        .as_ref()
        .context("压缩包任务缺少 cleanup_dir")?;
    if cleanup_dir.exists() {
        if !cleanup_dir.is_dir() {
            bail!("临时解压目录不是目录: {}", cleanup_dir.display());
        }
        fs::remove_dir_all(cleanup_dir)
            .with_context(|| format!("无法清理旧的临时目录: {}", cleanup_dir.display()))?;
    }
    fs::create_dir_all(cleanup_dir)
        .with_context(|| format!("无法创建临时解压目录: {}", cleanup_dir.display()))?;

    let archive_file = File::open(archive_path)
        .with_context(|| format!("无法打开压缩包: {}", archive_path.display()))?;
    let decoder = GzDecoder::new(BufReader::new(archive_file));
    let mut archive = Archive::new(decoder);
    for entry in archive
        .entries()
        .with_context(|| format!("无法读取压缩包内容: {}", archive_path.display()))?
    {
        let mut entry =
            entry.with_context(|| format!("压缩包条目读取失败: {}", archive_path.display()))?;
        entry
            .unpack_in(cleanup_dir)
            .with_context(|| format!("解压条目失败: {}", archive_path.display()))?;
    }

    Ok(true)
}

pub(crate) fn cleanup_consumed_task_input(task: &BatchTask) -> Result<()> {
    if task.input_kind != TaskInputKind::TarGzArchive {
        return Ok(());
    }

    if let Some(cleanup_dir) = &task.cleanup_dir {
        remove_dir_with_retry(cleanup_dir, "临时解压目录")?;
    }

    Ok(())
}

pub(crate) fn ensure_task_dirs(task: &BatchTask) -> Result<()> {
    if let Some(output_path) = &task.output_point_cloud {
        ensure_parent_dir(output_path)?;
    }
    ensure_parent_dir(&task.intensity_png)?;
    ensure_parent_dir(&task.utm_collected_path)?;
    ensure_parent_dir(&task.status_path)?;
    ensure_parent_dir(&task.package_log_path)?;
    Ok(())
}

pub(crate) fn required_outputs(
    task: &BatchTask,
    origin: Option<&Path>,
    output_format: PointCloudOutputFormat,
) -> Vec<PathBuf> {
    let mut outputs = vec![
        task.intensity_png.clone(),
        task.utm_collected_path.clone(),
        world_file_path(&task.intensity_png),
    ];
    if output_format.writes_point_cloud()
        && let Some(output_path) = &task.output_point_cloud
    {
        outputs.insert(0, output_path.clone());
    }
    if origin.is_some() {
        outputs.push(sidecar_path(&task.intensity_png, "prj"));
        outputs.push(aux_xml_path(&task.intensity_png));
        outputs.push(vrt_path(&task.intensity_png));
    }
    outputs
}

pub(crate) fn write_status_file(
    task: &BatchTask,
    report: &PcdProcessReport,
    origin: Option<&Path>,
    output_format: PointCloudOutputFormat,
    voxel_size: f64,
    intensity_resolution: f64,
) -> Result<()> {
    let payload = StatusFile {
        completed_at_epoch_s: unix_now(),
        dataset_name: task.dataset_name.clone(),
        dataset_dir: task.dataset_dir.display().to_string(),
        source_archive_path: task
            .source_archive_path
            .as_ref()
            .map(|path| path.display().to_string()),
        pcd_dir: task.pcd_dir.display().to_string(),
        enu_path: task.enu_path.display().to_string(),
        utm_path: task.utm_path.display().to_string(),
        output_point_cloud: task
            .output_point_cloud
            .as_ref()
            .map(|path| path.display().to_string()),
        output_format,
        intensity_png: task.intensity_png.display().to_string(),
        utm_collected_path: task.utm_collected_path.display().to_string(),
        package_log: task.package_log_path.display().to_string(),
        origin_path: origin.map(|path| path.display().to_string()),
        voxel_size,
        intensity_resolution,
        representative: representative_name(RepresentativeMode::Center),
        threads: report.threads_used,
        pivot: pivot_name(PivotMode::Centroid),
        mapping: mapping_name(MappingMode::Enu),
        output_enabled: output_format.writes_point_cloud(),
        report: report.clone(),
        required_outputs: required_outputs(task, origin, output_format)
            .into_iter()
            .map(|path| path.display().to_string())
            .collect(),
    };
    let json = serde_json::to_string_pretty(&payload).context("生成状态文件 JSON 失败")?;
    fs::write(&task.status_path, json)
        .with_context(|| format!("无法写出状态文件: {}", task.status_path.display()))?;
    Ok(())
}

fn voxel_output_tag(voxel_size: f64) -> String {
    if voxel_size <= 0.0 {
        return "fullres".to_string();
    }
    let centimeters = voxel_size * 100.0;
    if is_effectively_integer(centimeters) {
        return format!("{:.0}cm", centimeters.round());
    }
    let millimeters = voxel_size * 1000.0;
    if is_effectively_integer(millimeters) {
        return format!("{:.0}mm", millimeters.round());
    }
    format!("{}m", format_float(voxel_size).replace('.', "p"))
}

fn is_effectively_integer(value: f64) -> bool {
    (value - value.round()).abs() <= 1e-9
}

fn extracted_layout_ready(task: &BatchTask) -> bool {
    task.process_root.is_dir() && task.pcd_dir.is_dir() && task.enu_path.is_file()
}

fn remove_dir_with_retry(path: &Path, label: &str) -> Result<()> {
    if !path.exists() {
        return Ok(());
    }

    let mut last_error = None;
    for attempt in 1..=CLEANUP_RETRY_ATTEMPTS {
        match fs::remove_dir_all(path) {
            Ok(()) => return Ok(()),
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(()),
            Err(error) => {
                if !path.exists() {
                    return Ok(());
                }
                last_error = Some(anyhow!(error));
                if attempt < CLEANUP_RETRY_ATTEMPTS {
                    thread::sleep(Duration::from_millis(CLEANUP_RETRY_DELAY_MS));
                }
            }
        }
    }

    if schedule_background_delete(path)? {
        return Ok(());
    }

    let error = last_error.unwrap_or_else(|| anyhow!("未知目录删除错误"));
    Err(error).with_context(|| format!("无法删除{}: {}", label, path.display()))
}

fn schedule_background_delete(path: &Path) -> Result<bool> {
    if !path.exists() {
        return Ok(true);
    }

    #[cfg(windows)]
    {
        let path_literal = path.display().to_string().replace('\'', "''");
        let command = format!(
            "$path = '{path_literal}'; \
for ($i = 0; $i -lt 30; $i++) {{ \
  if (-not (Test-Path -LiteralPath $path)) {{ exit 0 }} \
  try {{ \
    Remove-Item -LiteralPath $path -Recurse -Force -ErrorAction Stop; \
    exit 0 \
  }} catch {{ \
    Start-Sleep -Seconds 2 \
  }} \
}} \
exit 1"
        );
        Command::new("powershell")
            .args([
                "-NoProfile",
                "-ExecutionPolicy",
                "Bypass",
                "-WindowStyle",
                "Hidden",
                "-Command",
                &command,
            ])
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .with_context(|| format!("无法启动后台清理进程: {}", path.display()))?;
        return Ok(true);
    }

    #[cfg(not(windows))]
    {
        Ok(false)
    }
}
