package bxn_point_cloud_renew

import (
	"github.com/im7mortal/UTM"
)

// lonToZone 根据经度推算 UTM 带号（1-60）。
func lonToZone(lon float64) int {
	z := int((lon+180.0)/6.0) + 1
	if z < 1 {
		z = 1
	}
	if z > 60 {
		z = 60
	}
	return z
}

// utmToLatLon 将 UTM 坐标转为 WGS84 经纬度（北半球）。
func utmToLatLon(easting, northing float64, zone int) (lat, lon float64, err error) {
	return UTM.ToLatLon(easting, northing, zone, "", true)
}

// pointInPolygon 判断点 (lon, lat) 是否在多边形内（ray casting 算法）。
// polygon 为 [lon, lat] 顶点数组，首尾可不闭合。
func pointInPolygon(lon, lat float64, polygon [][]float64) bool {
	n := len(polygon)
	if n < 3 {
		return false
	}
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := polygon[i][0], polygon[i][1]
		xj, yj := polygon[j][0], polygon[j][1]
		if ((yi > lat) != (yj > lat)) && (lon < (xj-xi)*(lat-yi)/(yj-yi)+xi) {
			inside = !inside
		}
		j = i
	}
	return inside
}

// pointInAnyFrame 判断点是否在任意一个框内。
func pointInAnyFrame(lon, lat float64, frames []taskFrame) bool {
	for _, f := range frames {
		if pointInPolygon(lon, lat, f.polygon) {
			return true
		}
	}
	return false
}
