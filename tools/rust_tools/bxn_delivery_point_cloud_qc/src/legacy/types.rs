use clap::ValueEnum;
use glam::{DQuat, DVec3};
use las::Point;
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

pub(crate) const WGS84_A: f64 = 6_378_137.0;
pub(crate) const WGS84_F: f64 = 1.0 / 298.257_223_563;
pub(crate) const UTM_K0: f64 = 0.9996;
pub(crate) const UTM_FALSE_EASTING: f64 = 500_000.0;
pub(crate) const UTM_FALSE_NORTHING_SOUTH: f64 = 10_000_000.0;

#[derive(Copy, Clone, Debug, Eq, PartialEq, ValueEnum)]
pub enum RepresentativeMode {
    First,
    Center,
}

#[derive(Copy, Clone, Debug, Eq, PartialEq, ValueEnum)]
pub enum PivotMode {
    Centroid,
    BboxCenter,
    Zero,
}

#[derive(Copy, Clone, Debug, Eq, PartialEq, ValueEnum)]
pub enum MappingMode {
    Enu,
    Flat,
}

#[derive(Copy, Clone, Debug, Eq, PartialEq, ValueEnum, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PointCloudOutputFormat {
    Laz,
    Las,
    None,
}

impl Default for PointCloudOutputFormat {
    fn default() -> Self {
        Self::Laz
    }
}

impl PointCloudOutputFormat {
    pub fn writes_point_cloud(self) -> bool {
        !matches!(self, Self::None)
    }

    pub fn file_extension(self) -> Option<&'static str> {
        match self {
            Self::Laz => Some("laz"),
            Self::Las => Some("las"),
            Self::None => None,
        }
    }

    pub fn dir_suffix(self) -> &'static str {
        match self {
            Self::Laz => "laz",
            Self::Las => "las",
            Self::None => "none",
        }
    }

    pub fn display_name(self) -> &'static str {
        match self {
            Self::Laz => "laz",
            Self::Las => "las",
            Self::None => "none",
        }
    }
}

#[derive(Copy, Clone, Debug, Eq, PartialEq, Hash)]
pub(crate) struct VoxelKey {
    pub(crate) x: i32,
    pub(crate) y: i32,
    pub(crate) z: i32,
}

#[derive(Debug)]
pub(crate) struct SelectedPoint {
    pub(crate) point: Point,
    pub(crate) score: f64,
    pub(crate) order: u64,
}

#[derive(Debug, Default, Clone)]
pub(crate) struct RasterCell {
    pub(crate) intensity_sum: f64,
    pub(crate) count: u32,
}

#[derive(Debug, Clone)]
pub(crate) struct RasterLayout {
    pub(crate) min_x: f64,
    pub(crate) min_y: f64,
    pub(crate) max_x: f64,
    pub(crate) max_y: f64,
    pub(crate) resolution: f64,
    pub(crate) width: usize,
    pub(crate) height: usize,
    pub(crate) pixel_count: usize,
}

#[derive(Debug, Clone, Default)]
pub(crate) struct PreviewBuildStats {
    pub(crate) width: usize,
    pub(crate) height: usize,
    pub(crate) non_empty_pixels: usize,
    pub(crate) accumulate_secs: f64,
    pub(crate) quantile_secs: f64,
    pub(crate) render_secs: f64,
    pub(crate) encode_secs: f64,
    pub(crate) sidecar_secs: f64,
    pub(crate) total_secs: f64,
}

#[derive(Debug, Clone, Default, Serialize)]
pub struct StageRuntimeReport {
    pub duration_secs: f64,
    pub peak_memory_bytes: u64,
}

#[derive(Debug, Clone, Default, Serialize)]
pub struct PipelineRuntimeReport {
    pub stage1_decode_voxel: StageRuntimeReport,
    pub stage2_transform: StageRuntimeReport,
    pub stage3_intensity_preview: StageRuntimeReport,
    pub stage4_laz_write: StageRuntimeReport,
    pub stage5_utm_collect: StageRuntimeReport,
    pub total_secs: f64,
    pub peak_memory_bytes: u64,
}

#[derive(Debug, Clone)]
pub(crate) struct OriginInfo {
    pub(crate) lat: f64,
    pub(crate) lon: f64,
    pub(crate) alt: f64,
}

#[derive(Debug, Clone)]
pub(crate) struct TransformConfig {
    pub(crate) origin: OriginInfo,
    pub(crate) epsg: u16,
    pub(crate) origin_utm: (f64, f64, f64),
    pub(crate) origin_ecef: (f64, f64, f64),
    pub(crate) pivot: (f64, f64),
    pub(crate) yaw_deg: f64,
    pub(crate) mapping: MappingMode,
}

#[derive(Debug, Clone)]
pub(crate) struct PivotAccumulator {
    pub(crate) count: u64,
    pub(crate) sum_x: f64,
    pub(crate) sum_y: f64,
    pub(crate) min_x: f64,
    pub(crate) min_y: f64,
    pub(crate) max_x: f64,
    pub(crate) max_y: f64,
}

#[derive(Debug, Clone)]
pub struct PcdProcessRequest {
    pub dataset_name: String,
    pub pcd_dir: PathBuf,
    pub enu_path: PathBuf,
    pub utm_path: PathBuf,
    pub output: Option<PathBuf>,
    pub output_format: PointCloudOutputFormat,
    pub intensity_preview: PathBuf,
    pub utm_output: PathBuf,
    pub voxel_size: f64,
    pub representative: RepresentativeMode,
    pub threads: usize,
    pub intensity_resolution: f64,
    pub origin: Option<PathBuf>,
    pub yaw_deg: f64,
    pub pivot: PivotMode,
    pub mapping: MappingMode,
    pub epsg: Option<u16>,
    pub force: bool,
    pub quiet: bool,
    pub log_path: Option<PathBuf>,
}

#[derive(Debug, Clone, Default, Serialize)]
pub struct PcdProcessReport {
    pub dataset_name: String,
    pub total_poses: usize,
    pub scanned_pcd_files: usize,
    pub valid_pcd_files: usize,
    pub skipped_pcd_files: usize,
    pub matched_frames: usize,
    pub failed_frames: usize,
    pub unmatched_poses: usize,
    pub input_points: u64,
    pub output_points: u64,
    pub threads_used: usize,
    pub runtime: PipelineRuntimeReport,
}

#[derive(Debug, Clone)]
pub enum PcdProcessOutcome {
    Success(PcdProcessReport),
    Skipped {
        report: PcdProcessReport,
        reason: String,
    },
}

#[derive(Clone, Debug)]
pub(crate) struct PoseSample {
    pub(crate) timestamp_text: String,
    pub(crate) translation: DVec3,
    pub(crate) rotation: DQuat,
}

#[derive(Clone, Debug)]
pub(crate) struct FrameEntry {
    pub(crate) timestamp: f64,
    pub(crate) path: PathBuf,
    pub(crate) pose_index: usize,
    pub(crate) point_count: usize,
    pub(crate) data_offset: u64,
    pub(crate) schema: PcdSchema,
}

#[derive(Clone, Debug)]
pub(crate) struct PcdCandidate {
    pub(crate) timestamp: f64,
    pub(crate) path: PathBuf,
    pub(crate) point_count: usize,
    pub(crate) data_offset: u64,
    pub(crate) schema: PcdSchema,
}

#[derive(Clone, Debug)]
pub(crate) struct PcdSchema {
    pub(crate) point_stride: usize,
    pub(crate) x_field: PcdField,
    pub(crate) y_field: PcdField,
    pub(crate) z_field: PcdField,
    pub(crate) intensity_field: Option<PcdField>,
}

#[derive(Clone, Copy, Debug)]
pub(crate) struct PcdField {
    pub(crate) kind: PcdFieldKind,
    pub(crate) size: usize,
    pub(crate) offset: usize,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub(crate) enum PcdFieldKind {
    Signed,
    Unsigned,
    Float,
}

#[derive(Debug)]
pub(crate) struct ParsedPcdHeader {
    pub(crate) schema: PcdSchema,
    pub(crate) points: usize,
    pub(crate) data_offset: u64,
    pub(crate) data_encoding: String,
}

#[derive(Debug)]
pub(crate) struct IndexedPoint {
    pub(crate) key: VoxelKey,
    pub(crate) point: Point,
    pub(crate) order: u64,
}

#[derive(Debug)]
pub(crate) struct ShardChunk {
    pub(crate) points: Vec<IndexedPoint>,
}

#[derive(Debug)]
pub(crate) struct ShardResult {
    pub(crate) selected: Vec<SelectedPoint>,
    pub(crate) pivot_acc: PivotAccumulator,
    pub(crate) processed_chunks: usize,
}

impl Default for PivotAccumulator {
    fn default() -> Self {
        Self {
            count: 0,
            sum_x: 0.0,
            sum_y: 0.0,
            min_x: f64::INFINITY,
            min_y: f64::INFINITY,
            max_x: f64::NEG_INFINITY,
            max_y: f64::NEG_INFINITY,
        }
    }
}
