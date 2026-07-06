use crate::legacy::cli::{configure_threads, validate_output_path};
use crate::legacy::io_utils::write_point_cloud;
use crate::legacy::logging::{ProcessLogger, log_process};
use crate::legacy::pcd::{
    build_pcd_header_template, load_and_transform_frame, load_pcd_poses, scan_pcd_candidates,
};
use crate::legacy::raster::{
    aux_xml_path, preview_format, sidecar_path, vrt_path, world_file_path,
    write_intensity_preview_points,
};
use crate::legacy::transform::{
    apply_transform, build_transform_config, build_transformed_header, merge_pivot_accumulator,
    pivot_from_accumulator, point_order, rebuild_header_for_points, update_pivot_accumulator,
    voxel_shard,
};
use crate::legacy::types::{
    IndexedPoint, PcdProcessOutcome, PcdProcessReport, PcdProcessRequest, PivotAccumulator,
    SelectedPoint, ShardChunk, ShardResult, StageRuntimeReport, VoxelKey,
};
use crate::legacy::voxel::{insert_selected_point_with_key, voxel_key};
use anyhow::{Result, anyhow, bail};
use crossbeam_channel::bounded;
use las::Point;
use rayon::prelude::*;
use rustc_hash::FxHashMap;
use std::fs;
use std::sync::atomic::{AtomicBool, AtomicU64, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Instant;

pub fn process_pcd_request(request: PcdProcessRequest) -> Result<PcdProcessOutcome> {
    run_pcd_pipeline(request)
}

fn run_pcd_pipeline(request: PcdProcessRequest) -> Result<PcdProcessOutcome> {
    if !(request.voxel_size.is_finite() && request.voxel_size >= 0.0) {
        bail!("voxel_size 必须是大于等于 0 的有限数值");
    }
    if !(request.intensity_resolution.is_finite() && request.intensity_resolution > 0.0) {
        bail!("intensity_resolution 必须是正数");
    }
    if request.threads == 0 {
        bail!("threads 必须大于 0");
    }

    let logger = Arc::new(Mutex::new(ProcessLogger::new(
        request.log_path.as_deref(),
        request.quiet,
    )?));
    let pipeline_started = Instant::now();
    let pipeline_memory_sampler = MemorySampler::start();
    log_process(
        &logger,
        "INFO",
        format!(
            "开始处理数据包 {}: pcd_dir={}, enu={}, utm={}, output={}, preview={}, utm_output={}, threads={}, skip_laz={}",
            request.dataset_name,
            request.pcd_dir.display(),
            request.enu_path.display(),
            request.utm_path.display(),
            request.output.display(),
            request.intensity_preview.display(),
            request.utm_output.display(),
            request.threads,
            request.skip_laz
        ),
    );

    configure_threads(Some(request.threads))?;
    preview_format(&request.intensity_preview)?;
    if !request.skip_laz {
        validate_output_path(&request.output, request.force, "输出文件")?;
    }
    validate_output_path(&request.intensity_preview, request.force, "强度预览图")?;
    validate_output_path(
        &world_file_path(&request.intensity_preview)?,
        request.force,
        "世界文件",
    )?;
    validate_output_path(
        &aux_xml_path(&request.intensity_preview),
        request.force,
        "Aux XML 文件",
    )?;
    validate_output_path(
        &vrt_path(&request.intensity_preview),
        request.force,
        "VRT 文件",
    )?;
    validate_output_path(
        &sidecar_path(&request.intensity_preview, "prj"),
        request.force,
        "PRJ 文件",
    )?;
    validate_output_path(&request.utm_output, request.force, "UTM 收集文件")?;

    let mut report = PcdProcessReport {
        dataset_name: request.dataset_name.clone(),
        threads_used: request.threads,
        ..PcdProcessReport::default()
    };

    let poses = match load_pcd_poses(&request.enu_path) {
        Ok(poses) => poses,
        Err(error) => {
            let reason = format!("ENU 文件无法解析: {error:#}");
            log_process(&logger, "WARN", &reason);
            finalize_pipeline_runtime(&mut report, pipeline_started, pipeline_memory_sampler)?;
            return Ok(PcdProcessOutcome::Skipped { report, reason });
        }
    };
    report.total_poses = poses.len();
    log_process(
        &logger,
        "INFO",
        format!("成功读取 {} 条 ENU 轨迹", poses.len()),
    );

    let scan = match scan_pcd_candidates(&request.pcd_dir, &logger) {
        Ok(scan) => scan,
        Err(error) => {
            let reason = format!("PCD 目录无法扫描: {error:#}");
            log_process(&logger, "WARN", &reason);
            finalize_pipeline_runtime(&mut report, pipeline_started, pipeline_memory_sampler)?;
            return Ok(PcdProcessOutcome::Skipped { report, reason });
        }
    };
    report.scanned_pcd_files = scan.scanned_pcd_files;
    report.valid_pcd_files = scan.valid_pcd_files;
    report.skipped_pcd_files = scan.skipped_pcd_files;
    if report.valid_pcd_files == 0 {
        let reason = format!(
            "没有可用 PCD 文件，扫描 {} 个，跳过 {} 个",
            report.scanned_pcd_files, report.skipped_pcd_files
        );
        log_process(&logger, "WARN", &reason);
        finalize_pipeline_runtime(&mut report, pipeline_started, pipeline_memory_sampler)?;
        return Ok(PcdProcessOutcome::Skipped { report, reason });
    }

    let mut candidates = scan.candidates;
    let mut frames = Vec::new();
    for (pose_index, pose) in poses.iter().enumerate() {
        if let Some(candidate) = candidates.remove(&pose.timestamp_text) {
            frames.push(crate::legacy::types::FrameEntry {
                timestamp: candidate.timestamp,
                path: candidate.path,
                pose_index,
                point_count: candidate.point_count,
                data_offset: candidate.data_offset,
                schema: candidate.schema,
            });
        } else {
            report.unmatched_poses += 1;
        }
    }
    report.matched_frames = frames.len();
    log_process(
        &logger,
        "INFO",
        format!(
            "轨迹匹配完成：有效 pcd={}，匹配帧={}，未匹配 ENU={}，多余 pcd={}",
            report.valid_pcd_files,
            report.matched_frames,
            report.unmatched_poses,
            candidates.len()
        ),
    );
    if frames.is_empty() {
        let reason = "没有任何 ENU 与有效 PCD 成功匹配".to_string();
        log_process(&logger, "WARN", &reason);
        finalize_pipeline_runtime(&mut report, pipeline_started, pipeline_memory_sampler)?;
        return Ok(PcdProcessOutcome::Skipped { report, reason });
    }

    log_process(
        &logger,
        "INFO",
        format!(
            "开始阶段 1/5：流式解码 + 并行体素抽稀，线程={}，shard={}，队列容量={}，待处理帧={}",
            request.threads,
            request.threads.max(1),
            (request.threads.max(1) * 2).max(8),
            report.matched_frames
        ),
    );
    let voxel_enabled = request.voxel_size > 0.0;
    let stage1_label = if voxel_enabled {
        "流式解码 + 并行体素抽稀"
    } else {
        "流式解码 + 并行保留全部点"
    };
    log_process(
        &logger,
        "INFO",
        format!(
            "阶段 1/5 模式：{}，voxel_size={}m",
            stage1_label, request.voxel_size
        ),
    );

    let template_header = build_pcd_header_template()?;
    let failed_frames = Arc::new(AtomicUsize::new(0));
    let decoded_frames = Arc::new(AtomicUsize::new(0));
    let input_points = Arc::new(AtomicU64::new(0));
    let selected_points_count = Arc::new(AtomicU64::new(0));
    let shard_count = request.threads.max(1);
    let queue_capacity = (request.threads.max(1) * 2).max(8);
    let poses_owned = poses.clone();
    let logger_for_workers = Arc::clone(&logger);
    let failed_frames_for_workers = Arc::clone(&failed_frames);
    let decoded_frames_for_workers = Arc::clone(&decoded_frames);
    let input_points_for_workers = Arc::clone(&input_points);
    let selected_points_for_workers = Arc::clone(&selected_points_count);
    let mut shard_receivers = Vec::with_capacity(shard_count);
    let mut shard_senders = Vec::with_capacity(shard_count);
    for _ in 0..shard_count {
        let (tx, rx) = bounded::<ShardChunk>(queue_capacity);
        shard_senders.push(tx);
        shard_receivers.push(rx);
    }

    let inv_voxel = voxel_enabled.then(|| 1.0 / request.voxel_size);
    let ((selected, pivot_acc, processed_chunks), stage1_runtime) = run_stage_with_memory(|| {
        let mut shard_workers = Vec::with_capacity(shard_count);
        for rx in shard_receivers {
            let selected_points_for_shard = Arc::clone(&selected_points_count);
            let voxel_size = request.voxel_size;
            let representative = request.representative;
            let voxel_enabled = voxel_enabled;
            shard_workers.push(thread::spawn(move || {
                let mut voxel_index: FxHashMap<VoxelKey, usize> = FxHashMap::default();
                let mut selected = Vec::<SelectedPoint>::new();
                let mut pivot_acc = PivotAccumulator::default();
                let mut processed_chunks = 0usize;
                for chunk in rx {
                    processed_chunks += 1;
                    for indexed in chunk.points {
                        update_pivot_accumulator(&mut pivot_acc, &indexed.point);
                        if voxel_enabled {
                            if insert_selected_point_with_key(
                                &mut voxel_index,
                                &mut selected,
                                indexed.key,
                                indexed.point,
                                voxel_size,
                                representative,
                                indexed.order,
                            ) {
                                selected_points_for_shard.fetch_add(1, Ordering::Relaxed);
                            }
                        } else {
                            selected.push(SelectedPoint {
                                point: indexed.point,
                                score: 0.0,
                                order: indexed.order,
                            });
                            selected_points_for_shard.fetch_add(1, Ordering::Relaxed);
                        }
                    }
                }
                ShardResult {
                    selected,
                    pivot_acc,
                    processed_chunks,
                }
            }));
        }

        let matched_frames = report.matched_frames;
        let producer = thread::spawn(move || -> Result<()> {
            frames.into_par_iter().try_for_each_init(
                || shard_senders.clone(),
                |senders, frame| -> Result<()> {
                    let pose = &poses_owned[frame.pose_index];
                    match load_and_transform_frame(&frame, pose) {
                        Ok(points) => {
                            let points_len = points.len();
                            let decoded = decoded_frames_for_workers.fetch_add(1, Ordering::Relaxed) + 1;
                            let cumulative_input = input_points_for_workers.fetch_add(points_len as u64, Ordering::Relaxed)
                                + points_len as u64;
                            let mut shard_batches: Vec<Vec<IndexedPoint>> =
                                (0..shard_count).map(|_| Vec::new()).collect();
                            for (point_index, point) in points.into_iter().enumerate() {
                                let order = point_order(frame.pose_index, point_index);
                                let (key, shard) = if let Some(inv_voxel) = inv_voxel {
                                    let key = voxel_key(&point, inv_voxel).map_err(|error| anyhow!(
                                        "{error:#}: frame={} ({}, {}, {})",
                                        frame.path.display(),
                                        point.x,
                                        point.y,
                                        point.z
                                    ))?;
                                    (key, voxel_shard(key, shard_count))
                                } else {
                                    (
                                        VoxelKey { x: 0, y: 0, z: 0 },
                                        (order % shard_count as u64) as usize,
                                    )
                                };
                                shard_batches[shard].push(IndexedPoint {
                                    key,
                                    point,
                                    order,
                                });
                            }
                            for (shard_idx, shard_points) in shard_batches.into_iter().enumerate() {
                                if shard_points.is_empty() {
                                    continue;
                                }
                                senders[shard_idx]
                                    .send(ShardChunk { points: shard_points })
                                    .map_err(|_| anyhow!("shard 聚合线程已停止接收点数据"))?;
                            }
                            if decoded == 1 || decoded % 100 == 0 {
                                log_process(&logger_for_workers, "INFO", format!(
                                    "阶段 1/5 进行中：已调度 {} / {} 帧，最近帧 {} @ {:.6}，本帧 {} 点，累计输入 {} 点，当前保留约 {} 点",
                                    decoded, matched_frames, frame.path.display(), frame.timestamp, points_len,
                                    cumulative_input, selected_points_for_workers.load(Ordering::Relaxed)
                                ));
                            }
                            Ok(())
                        }
                        Err(error) => {
                            failed_frames_for_workers.fetch_add(1, Ordering::Relaxed);
                            log_process(&logger_for_workers, "WARN", format!("跳过损坏 PCD {}: {error:#}", frame.path.display()));
                            Ok(())
                        }
                    }
                },
            )
        });

        producer
            .join()
            .map_err(|_| anyhow!("PCD 解码线程发生 panic"))??;
        let mut selected = Vec::<SelectedPoint>::new();
        let mut pivot_acc = PivotAccumulator::default();
        let mut processed_chunks = 0usize;
        for worker in shard_workers {
            let result = worker
                .join()
                .map_err(|_| anyhow!("体素 shard 聚合线程发生 panic"))?;
            processed_chunks += result.processed_chunks;
            merge_pivot_accumulator(&mut pivot_acc, &result.pivot_acc);
            selected.extend(result.selected);
        }
        Ok((selected, pivot_acc, processed_chunks))
    })?;

    report.failed_frames = failed_frames.load(Ordering::Relaxed);
    report.matched_frames = decoded_frames.load(Ordering::Relaxed);
    report.input_points = input_points.load(Ordering::Relaxed);
    report.runtime.stage1_decode_voxel = stage1_runtime;
    if report.input_points == 0 || selected.is_empty() {
        let reason = format!(
            "没有可输出的有效点，解码成功帧={}，失败帧={}",
            report.matched_frames, report.failed_frames
        );
        log_process(&logger, "WARN", &reason);
        finalize_pipeline_runtime(&mut report, pipeline_started, pipeline_memory_sampler)?;
        return Ok(PcdProcessOutcome::Skipped { report, reason });
    }
    log_process(
        &logger,
        "INFO",
        format!(
            "完成阶段 1/5：{}，用时 {:.2}s，峰值内存={}，成功帧={}，失败帧={}，输入点={}，保留点={}，shard 块={}",
            stage1_label,
            report.runtime.stage1_decode_voxel.duration_secs,
            format_memory_bytes(report.runtime.stage1_decode_voxel.peak_memory_bytes),
            report.matched_frames,
            report.failed_frames,
            report.input_points,
            selected.len(),
            processed_chunks
        ),
    );

    let mut selected_points: Vec<Point> = selected.into_iter().map(|item| item.point).collect();
    let output_header = if request.origin.is_some() {
        let pivot = pivot_from_accumulator(&pivot_acc, request.pivot);
        let config = build_transform_config(
            request.origin.as_deref().expect("origin 已存在"),
            request.epsg,
            request.yaw_deg,
            request.mapping,
            pivot,
        )?;
        log_process(
            &logger,
            "INFO",
            format!(
                "开始阶段 2/5：对保留点执行 origin 偏转，保留点={}，pivot=({:.3}, {:.3})，epsg=EPSG:{}",
                selected_points.len(),
                pivot.0,
                pivot.1,
                config.epsg
            ),
        );
        let (_, stage2_runtime) = run_stage_with_memory(|| {
            selected_points
                .par_iter_mut()
                .try_for_each(|point| apply_transform(point, &config))
        })?;
        report.runtime.stage2_transform = stage2_runtime;
        log_process(
            &logger,
            "INFO",
            format!(
                "完成阶段 2/5：origin 偏转，用时 {:.2}s，峰值内存={}，输出坐标系=EPSG:{}",
                report.runtime.stage2_transform.duration_secs,
                format_memory_bytes(report.runtime.stage2_transform.peak_memory_bytes),
                config.epsg
            ),
        );
        build_transformed_header(&template_header, &selected_points, config.epsg)?
    } else {
        log_process(
            &logger,
            "INFO",
            "跳过阶段 2/5：未提供 origin，保留点直接使用 ENU/局部坐标输出",
        );
        rebuild_header_for_points(&template_header, &selected_points)?
    };

    log_process(
        &logger,
        "INFO",
        format!(
            "开始阶段 3/5：并行聚合强度图，保留点={}，分辨率={}m，输出={}",
            selected_points.len(),
            request.intensity_resolution,
            request.intensity_preview.display()
        ),
    );
    let (preview_stats, stage3_runtime) = run_stage_with_memory(|| {
        write_intensity_preview_points(
            &request.intensity_preview,
            request.intensity_resolution,
            &selected_points,
            &output_header,
            request.force,
        )
    })?;
    report.runtime.stage3_intensity_preview = stage3_runtime;
    log_process(
        &logger,
        "INFO",
        format!(
            "完成阶段 3/5：强度图输出，用时 {:.2}s，峰值内存={}，PNG={}，栅格={}x{}，有效像素={}，累计 {:.2}s，拉伸 {:.2}s，成图 {:.2}s，编码 {:.2}s，侧车 {:.2}s",
            report.runtime.stage3_intensity_preview.duration_secs,
            format_memory_bytes(report.runtime.stage3_intensity_preview.peak_memory_bytes),
            request.intensity_preview.display(),
            preview_stats.width,
            preview_stats.height,
            preview_stats.non_empty_pixels,
            preview_stats.accumulate_secs,
            preview_stats.quantile_secs,
            preview_stats.render_secs,
            preview_stats.encode_secs,
            preview_stats.sidecar_secs
        ),
    );
    if request.skip_laz {
        log_process(
            &logger,
            "INFO",
            format!(
                "跳过阶段 4/5：按参数要求不写出 LAZ，目标路径={}",
                request.output.display()
            ),
        );
    } else {
        log_process(
            &logger,
            "INFO",
            format!(
                "开始阶段 4/5：写出 LAZ，输出点={}，路径={}",
                selected_points.len(),
                request.output.display()
            ),
        );
        let (_, stage4_runtime) = run_stage_with_memory(|| {
            write_point_cloud(
                &request.output,
                &output_header,
                &selected_points,
                request.force,
            )
        })?;
        report.runtime.stage4_laz_write = stage4_runtime;
        log_process(
            &logger,
            "INFO",
            format!(
                "完成阶段 4/5：LAZ 写出，用时 {:.2}s，峰值内存={}，路径={}",
                report.runtime.stage4_laz_write.duration_secs,
                format_memory_bytes(report.runtime.stage4_laz_write.peak_memory_bytes),
                request.output.display()
            ),
        );
    }
    log_process(
        &logger,
        "INFO",
        format!(
            "开始阶段 5/5：收集 UTM 文件，源={}，目标={}",
            request.utm_path.display(),
            request.utm_output.display()
        ),
    );
    let (_, stage5_runtime) = run_stage_with_memory(|| {
        if let Some(parent) = request.utm_output.parent() {
            fs::create_dir_all(parent)?;
        }
        fs::copy(&request.utm_path, &request.utm_output)
            .map(|_| ())
            .map_err(|error| {
                anyhow!(
                    "复制 utm.txt 失败: {} -> {}: {error}",
                    request.utm_path.display(),
                    request.utm_output.display()
                )
            })
    })?;
    report.runtime.stage5_utm_collect = stage5_runtime;
    log_process(
        &logger,
        "INFO",
        format!(
            "完成阶段 5/5：UTM 文件收集，用时 {:.2}s，峰值内存={}，路径={}",
            report.runtime.stage5_utm_collect.duration_secs,
            format_memory_bytes(report.runtime.stage5_utm_collect.peak_memory_bytes),
            request.utm_output.display()
        ),
    );

    report.output_points = selected_points.len() as u64;
    finalize_pipeline_runtime(&mut report, pipeline_started, pipeline_memory_sampler)?;
    log_process(
        &logger,
        "INFO",
        format!(
            "数据包 {} 完成：输入 {} 点 -> 输出 {} 点，成功帧={}，失败帧={}，总用时 {:.2}s，峰值内存={}",
            request.dataset_name,
            report.input_points,
            report.output_points,
            report.matched_frames,
            report.failed_frames,
            report.runtime.total_secs,
            format_memory_bytes(report.runtime.peak_memory_bytes)
        ),
    );
    Ok(PcdProcessOutcome::Success(report))
}

struct MemorySampler {
    stop: Arc<AtomicBool>,
    handle: Option<thread::JoinHandle<u64>>,
}

impl MemorySampler {
    fn start() -> Self {
        let stop = Arc::new(AtomicBool::new(false));
        let stop_for_thread = Arc::clone(&stop);
        let handle = thread::spawn(move || {
            let mut peak = current_process_memory_bytes().unwrap_or(0);
            while !stop_for_thread.load(Ordering::Relaxed) {
                peak = peak.max(current_process_memory_bytes().unwrap_or(0));
                thread::sleep(std::time::Duration::from_millis(50));
            }
            peak.max(current_process_memory_bytes().unwrap_or(0))
        });
        Self {
            stop,
            handle: Some(handle),
        }
    }

    fn finish(mut self) -> Result<u64> {
        self.stop.store(true, Ordering::Relaxed);
        self.handle
            .take()
            .expect("memory sampler handle missing")
            .join()
            .map_err(|_| anyhow!("内存采样线程发生 panic"))
    }
}

fn run_stage_with_memory<T, F>(action: F) -> Result<(T, StageRuntimeReport)>
where
    F: FnOnce() -> Result<T>,
{
    let started = Instant::now();
    let sampler = MemorySampler::start();
    let result = action();
    let peak_memory_bytes = sampler.finish()?;
    let duration_secs = started.elapsed().as_secs_f64();
    result.map(|value| {
        (
            value,
            StageRuntimeReport {
                duration_secs,
                peak_memory_bytes,
            },
        )
    })
}

fn finalize_pipeline_runtime(
    report: &mut PcdProcessReport,
    pipeline_started: Instant,
    pipeline_memory_sampler: MemorySampler,
) -> Result<()> {
    report.runtime.total_secs = pipeline_started.elapsed().as_secs_f64();
    let stage_peak = report
        .runtime
        .stage1_decode_voxel
        .peak_memory_bytes
        .max(report.runtime.stage2_transform.peak_memory_bytes)
        .max(report.runtime.stage3_intensity_preview.peak_memory_bytes)
        .max(report.runtime.stage4_laz_write.peak_memory_bytes)
        .max(report.runtime.stage5_utm_collect.peak_memory_bytes);
    report.runtime.peak_memory_bytes = pipeline_memory_sampler.finish()?.max(stage_peak);
    Ok(())
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

fn current_process_memory_bytes() -> Option<u64> {
    #[cfg(target_os = "linux")]
    {
        let status = std::fs::read_to_string("/proc/self/status").ok()?;
        for line in status.lines() {
            if let Some(value) = line.strip_prefix("VmRSS:") {
                let kib = value.split_whitespace().next()?.parse::<u64>().ok()?;
                return Some(kib.saturating_mul(1024));
            }
        }
        None
    }

    #[cfg(windows)]
    {
        use std::mem::size_of;
        use windows_sys::Win32::System::ProcessStatus::{
            K32GetProcessMemoryInfo, PROCESS_MEMORY_COUNTERS,
        };
        use windows_sys::Win32::System::Threading::GetCurrentProcess;

        unsafe {
            let process = GetCurrentProcess();
            let mut counters = PROCESS_MEMORY_COUNTERS {
                cb: size_of::<PROCESS_MEMORY_COUNTERS>() as u32,
                PageFaultCount: 0,
                PeakWorkingSetSize: 0,
                WorkingSetSize: 0,
                QuotaPeakPagedPoolUsage: 0,
                QuotaPagedPoolUsage: 0,
                QuotaPeakNonPagedPoolUsage: 0,
                QuotaNonPagedPoolUsage: 0,
                PagefileUsage: 0,
                PeakPagefileUsage: 0,
            };
            if K32GetProcessMemoryInfo(
                process,
                &mut counters as *mut PROCESS_MEMORY_COUNTERS as *mut _,
                size_of::<PROCESS_MEMORY_COUNTERS>() as u32,
            ) == 0
            {
                None
            } else {
                Some(counters.WorkingSetSize as u64)
            }
        }
    }

    #[cfg(target_os = "macos")]
    {
        use libc::{
            KERN_SUCCESS, MACH_TASK_BASIC_INFO, MACH_TASK_BASIC_INFO_COUNT,
            integer_t, mach_msg_type_number_t, mach_task_basic_info_data_t, mach_task_self_,
            task_info, task_info_t,
        };

        unsafe {
            let mut info = std::mem::zeroed::<mach_task_basic_info_data_t>();
            let mut count = MACH_TASK_BASIC_INFO_COUNT as mach_msg_type_number_t;
            if task_info(
                mach_task_self_,
                MACH_TASK_BASIC_INFO,
                (&mut info as *mut mach_task_basic_info_data_t).cast::<integer_t>() as task_info_t,
                &mut count,
            ) != KERN_SUCCESS
            {
                None
            } else {
                Some(info.resident_size as u64)
            }
        }
    }

    #[cfg(not(any(target_os = "linux", target_os = "macos", windows)))]
    {
        None
    }
}
