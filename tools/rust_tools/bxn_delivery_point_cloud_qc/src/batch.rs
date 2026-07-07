mod discovery;
mod ledger;
mod paths;
mod types;

use crate::legacy::{
    self, MappingMode, PcdProcessOutcome, PcdProcessRequest, PivotMode, PointCloudOutputFormat,
    RepresentativeMode,
};
use anyhow::{Context, Result, anyhow, bail};
use clap::Parser;
use discovery::{
    cleanup_consumed_task_input, discover_tasks, ensure_task_dirs, prepare_task_input,
    validate_task_layout, write_status_file,
};
use ledger::{
    compute_package_fingerprint, flush_ledger, load_or_init_ledger, prepare_output_root,
    should_skip_by_ledger,
};
use paths::{
    format_float, paths_equivalent, resolve_output_root, resolve_user_path, unix_now,
    validate_origin_file,
};
use std::thread::{self, JoinHandle};
use types::{
    BatchCli, BatchLogger, BatchTask, DEFAULT_INTENSITY_RESOLUTION, DEFAULT_LEDGER_NAME,
    LedgerPackageEntry, LedgerStatus, TaskInputKind, mapping_name, pivot_name, representative_name,
};

struct PendingPrefetch {
    index: usize,
    dataset_name: String,
    handle: JoinHandle<Result<bool>>,
}

struct PackageRunSummary {
    dataset_name: String,
    status: &'static str,
    report: Option<legacy::PcdProcessReport>,
    message: Option<String>,
}

pub fn run_batch_cli_from(args: &[String]) -> Result<()> {
    let cli = BatchCli::try_parse_from(
        std::iter::once("bxn_delivery_point_cloud_qc").chain(args.iter().map(String::as_str)),
    )?;
    run_batch(cli)
}

fn run_batch(cli: BatchCli) -> Result<()> {
    validate_cli(&cli)?;
    let input_root = resolve_user_path(&cli.input)?;
    let output_root = resolve_output_root(&cli.output)?;
    let global_origin = cli
        .origin
        .as_ref()
        .map(|path| resolve_user_path(path))
        .transpose()?;
    if paths_equivalent(&input_root, &output_root) {
        bail!("--input 和 --output 不能指向同一个目录");
    }

    let ledger_path = match &cli.ledger {
        Some(path) => {
            let resolved = resolve_user_path(path)?;
            if !resolved.is_file() {
                bail!(
                    "--ledger 指向的台账不存在或不是文件: {}",
                    resolved.display()
                );
            }
            resolved
        }
        None => output_root.join(DEFAULT_LEDGER_NAME),
    };
    prepare_output_root(&output_root, &ledger_path, cli.ledger.is_some())?;

    let batch_log_path = output_root.join("logs").join("batch.log");
    let mut logger = BatchLogger::new(&batch_log_path)?;
    logger.log("INFO", format!("输入目录: {}", input_root.display()));
    logger.log("INFO", format!("输出目录: {}", output_root.display()));
    logger.log(
        "INFO",
        format!(
            "处理参数: voxel={} intensity={} representative={} threads={} pivot={} mapping={} origin={} point_cloud_output={}",
            format_float(cli.voxel_size),
            format_float(DEFAULT_INTENSITY_RESOLUTION),
            representative_name(RepresentativeMode::Center),
            cli.threads,
            pivot_name(PivotMode::Centroid),
            mapping_name(MappingMode::Enu),
            global_origin
                .as_ref()
                .map(|path| path.display().to_string())
                .unwrap_or_else(|| "none".to_string()),
            cli.output_format.display_name()
        ),
    );
    logger.log("INFO", format!("台账路径: {}", ledger_path.display()));

    let mut ledger = load_or_init_ledger(
        &ledger_path,
        &input_root,
        &output_root,
        global_origin.as_deref(),
    )?;
    let tasks = discover_tasks(&input_root, &output_root, cli.voxel_size, cli.output_format)?;
    if tasks.is_empty() {
        logger.log("WARN", "未找到符合命名规范的数据包目录");
        bail!("没有可处理的数据包");
    }
    logger.log("INFO", format!("命中 {} 个候选数据包", tasks.len()));

    let mut success_count = 0usize;
    let mut skipped_count = 0usize;
    let mut failed_count = 0usize;
    let mut ledger_skip_count = 0usize;
    let mut pending_prefetch = None;
    let mut package_summaries = Vec::new();

    for (index, task) in tasks.iter().cloned().enumerate() {
        let fingerprint = compute_package_fingerprint(
            &task,
            global_origin.as_deref(),
            cli.voxel_size,
            cli.output_format,
        )
        .ok();
        if should_skip_by_ledger(&task, fingerprint.as_ref(), &ledger, cli.output_format) {
            ledger_skip_count += 1;
            logger.log(
                "SKIP",
                format!(
                    "{} 命中台账，输入未变化且当前输出模式所需产物齐全（point_cloud_output={}），跳过",
                    task.label(),
                    cli.output_format.display_name()
                ),
            );
            package_summaries.push(PackageRunSummary {
                dataset_name: task.dataset_name.clone(),
                status: "ledger-skipped",
                report: None,
                message: Some(format!(
                    "命中台账，输入未变化且当前输出模式所需产物齐全（point_cloud_output={}）",
                    cli.output_format.display_name()
                )),
            });
            continue;
        }

        logger.log(
            "START",
            format!(
                "{} -> pcd_dir={} enu={}",
                task.label(),
                task.pcd_dir.display(),
                task.enu_path.display()
            ),
        );

        let input_prepare_result =
            join_prefetch_if_ready(&mut pending_prefetch, index, &mut logger)?
                .unwrap_or_else(|| prepare_task_input(&task));
        match input_prepare_result {
            Ok(true) => {
                logger.log(
                    "INFO",
                    format!(
                        "{} 已解压任务包到 {}",
                        task.label(),
                        task.process_root.display()
                    ),
                );
            }
            Ok(false) => {}
            Err(error) => {
                log_task_cleanup_warning(&task, &mut logger);
                failed_count += 1;
                let message = format!("{error:#}");
                logger.log("FAIL", format!("{}：{}", task.label(), message));
                ledger.packages.insert(
                    task.dataset_name.clone(),
                    LedgerPackageEntry {
                        status: LedgerStatus::Failed,
                        last_run_epoch_s: unix_now(),
                        message: message.clone(),
                        fingerprint,
                        output_point_cloud: task
                            .output_point_cloud
                            .as_ref()
                            .map(|path| path.display().to_string()),
                        output_format: task.output_format,
                        intensity_png: task.intensity_png.display().to_string(),
                    },
                );
                package_summaries.push(PackageRunSummary {
                    dataset_name: task.dataset_name.clone(),
                    status: "failed",
                    report: None,
                    message: Some(message),
                });
                flush_ledger(&ledger_path, &mut ledger)?;
                continue;
            }
        }

        match validate_task_layout(&task) {
            Ok(()) => {}
            Err(error) => {
                let cleanup_warning = log_task_cleanup_warning(&task, &mut logger);
                skipped_count += 1;
                let message = format_task_message(format!("{:#}", error), cleanup_warning);
                logger.log("SKIP", format!("{}：{}", task.label(), message));
                ledger.packages.insert(
                    task.dataset_name.clone(),
                    LedgerPackageEntry {
                        status: LedgerStatus::Skipped,
                        last_run_epoch_s: unix_now(),
                        message: message.clone(),
                        fingerprint,
                        output_point_cloud: task
                            .output_point_cloud
                            .as_ref()
                            .map(|path| path.display().to_string()),
                        output_format: task.output_format,
                        intensity_png: task.intensity_png.display().to_string(),
                    },
                );
                package_summaries.push(PackageRunSummary {
                    dataset_name: task.dataset_name.clone(),
                    status: "skipped",
                    report: None,
                    message: Some(message),
                });
                flush_ledger(&ledger_path, &mut ledger)?;
                continue;
            }
        }

        ensure_task_dirs(&task)?;
        maybe_start_prefetch(
            index + 1,
            &tasks,
            &ledger,
            global_origin.as_deref(),
            cli.output_format,
            cli.voxel_size,
            &mut pending_prefetch,
            &mut logger,
        );
        let request = PcdProcessRequest {
            dataset_name: task.dataset_name.clone(),
            pcd_dir: task.pcd_dir.clone(),
            enu_path: task.enu_path.clone(),
            utm_path: task.utm_path.clone(),
            output: task.output_point_cloud.clone(),
            output_format: cli.output_format,
            intensity_preview: task.intensity_png.clone(),
            utm_output: task.utm_collected_path.clone(),
            voxel_size: cli.voxel_size,
            representative: RepresentativeMode::Center,
            threads: cli.threads,
            intensity_resolution: DEFAULT_INTENSITY_RESOLUTION,
            origin: global_origin.clone(),
            yaw_deg: 0.0,
            pivot: PivotMode::Centroid,
            mapping: MappingMode::Enu,
            epsg: None,
            force: true,
            quiet: false,
            log_path: Some(task.package_log_path.clone()),
        };

        match legacy::process_pcd_request(request)
            .with_context(|| format!("执行数据包 {} 时出现异常", task.dataset_name))
        {
            Ok(PcdProcessOutcome::Success(report)) => {
                let cleanup_warning = log_task_cleanup_warning(&task, &mut logger);
                if let Err(error) = write_status_file(
                    &task,
                    &report,
                    global_origin.as_deref(),
                    cli.output_format,
                    cli.voxel_size,
                ) {
                    failed_count += 1;
                    let message = format_task_message(format!("{error:#}"), cleanup_warning);
                    logger.log("FAIL", format!("{}：{}", task.label(), message));
                    ledger.packages.insert(
                        task.dataset_name.clone(),
                        LedgerPackageEntry {
                            status: LedgerStatus::Failed,
                            last_run_epoch_s: unix_now(),
                            message: message.clone(),
                            fingerprint,
                            output_point_cloud: task
                                .output_point_cloud
                                .as_ref()
                                .map(|path| path.display().to_string()),
                            output_format: task.output_format,
                            intensity_png: task.intensity_png.display().to_string(),
                        },
                    );
                    package_summaries.push(PackageRunSummary {
                        dataset_name: task.dataset_name.clone(),
                        status: "failed",
                        report: Some(report),
                        message: Some(message),
                    });
                } else {
                    success_count += 1;
                    let ledger_message = cleanup_warning.clone().unwrap_or_else(|| {
                        format!(
                            "success: matched_frames={} input_points={} output_points={}",
                            report.matched_frames, report.input_points, report.output_points
                        )
                    });
                    logger.log(
                        " OK ",
                        format!(
                            "{}：匹配 {} 帧，输入 {} 点，输出 {} 点，日志 {}",
                            task.label(),
                            report.matched_frames,
                            report.input_points,
                            report.output_points,
                            task.package_log_path.display()
                        ),
                    );
                    ledger.packages.insert(
                        task.dataset_name.clone(),
                        LedgerPackageEntry {
                            status: LedgerStatus::Success,
                            last_run_epoch_s: unix_now(),
                            message: ledger_message,
                            fingerprint: fingerprint.clone().or_else(|| {
                                compute_package_fingerprint(
                                    &task,
                                    global_origin.as_deref(),
                                    cli.voxel_size,
                                    cli.output_format,
                                )
                                .ok()
                            }),
                            output_point_cloud: task
                                .output_point_cloud
                                .as_ref()
                                .map(|path| path.display().to_string()),
                            output_format: task.output_format,
                            intensity_png: task.intensity_png.display().to_string(),
                        },
                    );
                    package_summaries.push(PackageRunSummary {
                        dataset_name: task.dataset_name.clone(),
                        status: "success",
                        report: Some(report),
                        message: cleanup_warning,
                    });
                }
            }
            Ok(PcdProcessOutcome::Skipped { report, reason }) => {
                let cleanup_warning = log_task_cleanup_warning(&task, &mut logger);
                skipped_count += 1;
                let message = format_task_message(reason, cleanup_warning);
                logger.log(
                    "SKIP",
                    format!(
                        "{}：{}（有效 pcd={}，匹配帧={}，日志 {}）",
                        task.label(),
                        message,
                        report.valid_pcd_files,
                        report.matched_frames,
                        task.package_log_path.display()
                    ),
                );
                ledger.packages.insert(
                    task.dataset_name.clone(),
                    LedgerPackageEntry {
                        status: LedgerStatus::Skipped,
                        last_run_epoch_s: unix_now(),
                        message: message.clone(),
                        fingerprint,
                        output_point_cloud: task
                            .output_point_cloud
                            .as_ref()
                            .map(|path| path.display().to_string()),
                        output_format: task.output_format,
                        intensity_png: task.intensity_png.display().to_string(),
                    },
                );
                package_summaries.push(PackageRunSummary {
                    dataset_name: task.dataset_name.clone(),
                    status: "skipped",
                    report: Some(report),
                    message: Some(message),
                });
            }
            Err(error) => {
                let cleanup_warning = log_task_cleanup_warning(&task, &mut logger);
                failed_count += 1;
                let message = format_task_message(format!("{error:#}"), cleanup_warning);
                logger.log("FAIL", format!("{}：{}", task.label(), message));
                ledger.packages.insert(
                    task.dataset_name.clone(),
                    LedgerPackageEntry {
                        status: LedgerStatus::Failed,
                        last_run_epoch_s: unix_now(),
                        message: message.clone(),
                        fingerprint,
                        output_point_cloud: task
                            .output_point_cloud
                            .as_ref()
                            .map(|path| path.display().to_string()),
                        output_format: task.output_format,
                        intensity_png: task.intensity_png.display().to_string(),
                    },
                );
                package_summaries.push(PackageRunSummary {
                    dataset_name: task.dataset_name.clone(),
                    status: "failed",
                    report: None,
                    message: Some(message),
                });
            }
        }
        flush_ledger(&ledger_path, &mut ledger)?;
    }

    logger.log(
        "INFO",
        format!(
            "批次完成：成功 {}，跳过 {}，失败 {}，台账跳过 {}",
            success_count, skipped_count, failed_count, ledger_skip_count
        ),
    );
    logger.log("INFO", "任务汇总：");
    for summary in &package_summaries {
        logger.log("INFO", format_package_summary(summary));
    }
    if success_count == 0 && failed_count == 0 && (skipped_count > 0 || ledger_skip_count > 0) {
        return Ok(());
    }
    if success_count == 0 {
        bail!("没有任何数据包成功输出");
    }
    Ok(())
}

fn format_package_summary(summary: &PackageRunSummary) -> String {
    match &summary.report {
        Some(report) => format!(
            "{} | status={} | total={:.2}s | stage1={:.2}s/{} | stage2={:.2}s/{} | stage3={:.2}s/{} | stage4={:.2}s/{} | stage5={:.2}s/{} | peak_mem={} | pcd={} | points={} -> {}{}",
            summary.dataset_name,
            summary.status,
            report.runtime.total_secs,
            report.runtime.stage1_decode_voxel.duration_secs,
            format_memory_bytes(report.runtime.stage1_decode_voxel.peak_memory_bytes),
            report.runtime.stage2_transform.duration_secs,
            format_memory_bytes(report.runtime.stage2_transform.peak_memory_bytes),
            report.runtime.stage3_intensity_preview.duration_secs,
            format_memory_bytes(report.runtime.stage3_intensity_preview.peak_memory_bytes),
            report.runtime.stage4_laz_write.duration_secs,
            format_memory_bytes(report.runtime.stage4_laz_write.peak_memory_bytes),
            report.runtime.stage5_utm_collect.duration_secs,
            format_memory_bytes(report.runtime.stage5_utm_collect.peak_memory_bytes),
            format_memory_bytes(report.runtime.peak_memory_bytes),
            report.matched_frames,
            report.input_points,
            report.output_points,
            summary
                .message
                .as_ref()
                .map(|message| format!(" | note={message}"))
                .unwrap_or_default()
        ),
        None => format!(
            "{} | status={}{}",
            summary.dataset_name,
            summary.status,
            summary
                .message
                .as_ref()
                .map(|message| format!(" | note={message}"))
                .unwrap_or_default()
        ),
    }
}

fn format_memory_bytes(bytes: u64) -> String {
    const UNITS: [&str; 5] = ["B", "KiB", "MiB", "GiB", "TiB"];
    let mut value = bytes as f64;
    let mut unit_index = 0usize;
    while value >= 1024.0 && unit_index < UNITS.len() - 1 {
        value /= 1024.0;
        unit_index += 1;
    }
    if unit_index == 0 {
        format!("{} {}", bytes, UNITS[unit_index])
    } else {
        format!("{value:.2} {}", UNITS[unit_index])
    }
}

fn validate_cli(cli: &BatchCli) -> Result<()> {
    if cli.threads == 0 {
        bail!("--threads 必须大于 0");
    }
    if !cli.voxel_size.is_finite() || cli.voxel_size < 0.0 {
        bail!("--voxel-size 必须是大于等于 0 的有限数值");
    }
    if let Some(origin) = &cli.origin {
        validate_origin_file(origin)?;
    }
    Ok(())
}

fn join_prefetch_if_ready(
    pending_prefetch: &mut Option<PendingPrefetch>,
    index: usize,
    logger: &mut BatchLogger,
) -> Result<Option<Result<bool>>> {
    let Some(job) = pending_prefetch.take() else {
        return Ok(None);
    };
    if job.index != index {
        *pending_prefetch = Some(job);
        return Ok(None);
    }

    let result = job
        .handle
        .join()
        .map_err(|_| anyhow!("后台预解压线程发生 panic: {}", job.dataset_name))?;
    match &result {
        Ok(true) => logger.log("INFO", format!("{} 后台预解压完成", job.dataset_name)),
        Ok(false) => logger.log("INFO", format!("{} 后台预解压已就绪", job.dataset_name)),
        Err(error) => logger.log(
            "WARN",
            format!(
                "{} 后台预解压失败，将回退到主流程处理：{error:#}",
                job.dataset_name
            ),
        ),
    }
    Ok(Some(result))
}

fn maybe_start_prefetch(
    next_index: usize,
    tasks: &[BatchTask],
    ledger: &types::LedgerFile,
    origin: Option<&std::path::Path>,
    output_format: PointCloudOutputFormat,
    voxel_size: f64,
    pending_prefetch: &mut Option<PendingPrefetch>,
    logger: &mut BatchLogger,
) {
    if pending_prefetch.is_some() {
        return;
    }
    let Some(task) = tasks.get(next_index).cloned() else {
        return;
    };
    if task.input_kind != TaskInputKind::TarGzArchive {
        return;
    }

    let fingerprint = compute_package_fingerprint(&task, origin, voxel_size, output_format).ok();
    if should_skip_by_ledger(&task, fingerprint.as_ref(), ledger, output_format) {
        return;
    }

    logger.log(
        "INFO",
        format!(
            "{} 处理中，后台开始预解压下一个任务 {}",
            tasks[next_index.saturating_sub(1)].label(),
            task.label()
        ),
    );
    let dataset_name = task.dataset_name.clone();
    let handle = thread::spawn(move || prepare_task_input(&task));
    *pending_prefetch = Some(PendingPrefetch {
        index: next_index,
        dataset_name,
        handle,
    });
}

fn log_task_cleanup_warning(task: &BatchTask, logger: &mut BatchLogger) -> Option<String> {
    cleanup_consumed_task_input(task).err().map(|error| {
        let message = format!("{error:#}");
        logger.log(
            "WARN",
            format!("{} 清理临时解压目录失败：{}", task.label(), message),
        );
        message
    })
}

fn format_task_message(message: String, cleanup_warning: Option<String>) -> String {
    match cleanup_warning {
        Some(cleanup_warning) => format!("{message} | cleanup_warning={cleanup_warning}"),
        None => message,
    }
}
