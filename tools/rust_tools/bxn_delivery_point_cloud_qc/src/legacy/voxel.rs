use crate::legacy::types::{RepresentativeMode, SelectedPoint, VoxelKey};
use anyhow::{Result, bail};
use las::Point;
use rustc_hash::FxHashMap;
use std::collections::hash_map::Entry;

pub(crate) fn insert_selected_point_with_key(
    voxel_index: &mut FxHashMap<VoxelKey, usize>,
    selected: &mut Vec<SelectedPoint>,
    key: VoxelKey,
    point: Point,
    voxel_size: f64,
    representative: RepresentativeMode,
    order: u64,
) -> bool {
    let score = score_point(&point, key, voxel_size, representative);
    match voxel_index.entry(key) {
        Entry::Occupied(entry) => {
            let idx = *entry.get();
            let replace = match representative {
                RepresentativeMode::First => order < selected[idx].order,
                RepresentativeMode::Center => score < selected[idx].score,
            };
            if replace {
                selected[idx] = SelectedPoint {
                    point,
                    score,
                    order,
                };
            }
            false
        }
        Entry::Vacant(entry) => {
            let idx = selected.len();
            entry.insert(idx);
            selected.push(SelectedPoint {
                point,
                score,
                order,
            });
            true
        }
    }
}

pub(crate) fn voxel_key(point: &Point, inv_voxel: f64) -> Result<VoxelKey> {
    Ok(VoxelKey {
        x: quantize(point.x, inv_voxel)?,
        y: quantize(point.y, inv_voxel)?,
        z: quantize(point.z, inv_voxel)?,
    })
}

pub(crate) fn quantize(value: f64, inv_voxel: f64) -> Result<i32> {
    let index = (value * inv_voxel).floor();
    if !(i32::MIN as f64..=i32::MAX as f64).contains(&index) {
        bail!("量化后的体素索引超出 i32 范围: {}", index);
    }
    Ok(index as i32)
}

fn score_point(point: &Point, key: VoxelKey, voxel_size: f64, mode: RepresentativeMode) -> f64 {
    match mode {
        RepresentativeMode::First => 0.0,
        RepresentativeMode::Center => {
            let cx = (f64::from(key.x) + 0.5) * voxel_size;
            let cy = (f64::from(key.y) + 0.5) * voxel_size;
            let cz = (f64::from(key.z) + 0.5) * voxel_size;
            let dx = point.x - cx;
            let dy = point.y - cy;
            let dz = point.z - cz;
            dx * dx + dy * dy + dz * dz
        }
    }
}
