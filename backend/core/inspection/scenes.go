package inspection

import (
	"math"
	"strings"
	"time"
)

// 离岗（post_absence）的产品口径是「区域内连续 5 秒无人即判定离岗」。
// 该口径由确认时长、观察窗口、最少有效帧数三处共同保证；三者任一放大都会把
// 单路摄像机的判定时间重新拉长到几十秒，因此集中在这里定义，前后端共用同一组数值。
const (
	absenceConfirmSeconds = 5
	absenceWindowSeconds  = 10
	absenceMinFrames      = 5
	// 观察窗口必须比确认时长多出一段掉帧余量。离岗只需要覆盖推理抖动，睡岗依赖
	// 姿态分析且有 10 秒预热，仍需更宽的窗口。
	absenceWindowMarginSeconds = 5
	defaultWindowMarginSeconds = 15
)

func IsObjectDetectionScene(code string) bool {
	return code == "object_present" || code == "object_absent"
}

func minConfirmSeconds(code string) int {
	if code == "post_absence" {
		return absenceConfirmSeconds
	}
	return 10
}

func minWindowSeconds(s Scene) int {
	if s.Code == "post_absence" {
		return s.ConfirmSeconds + absenceWindowMarginSeconds
	}
	return s.ConfirmSeconds + defaultWindowMarginSeconds
}

func NormalizeScene(s *Scene) error {
	if strings.TrimSpace(s.Target) == "" || s.Confidence < .1 || s.Confidence > 1 || math.IsNaN(float64(s.Confidence)) {
		return ErrInvalid
	}
	if s.ROI != nil {
		r := s.ROI
		for _, v := range []float32{r.Left, r.Top, r.Right, r.Bottom} {
			if v < 0 || v > 1 || math.IsNaN(float64(v)) {
				return ErrInvalid
			}
		}
		if r.Right <= r.Left || r.Bottom <= r.Top {
			return ErrInvalid
		}
	}
	if IsObjectDetectionScene(s.Code) {
		return nil
	}
	if (s.Code != "post_absence" && s.Code != "sleep_on_duty") || s.Target != "person" {
		return ErrInvalid
	}
	if s.ConfirmSeconds == 0 {
		s.ConfirmSeconds = 30
		if s.Code == "post_absence" {
			s.ConfirmSeconds = absenceConfirmSeconds
		}
	}
	if s.WindowSeconds == 0 {
		if s.Code == "post_absence" {
			s.WindowSeconds = s.ConfirmSeconds + absenceWindowMarginSeconds
		} else {
			s.WindowSeconds = s.ConfirmSeconds + 30
		}
	}
	if s.MinFrames == 0 {
		s.MinFrames = 15
		if s.Code == "post_absence" {
			s.MinFrames = absenceMinFrames
		}
	}
	if s.MinPeople == 0 {
		s.MinPeople = 1
	}
	if s.ConfirmSeconds < minConfirmSeconds(s.Code) || s.ConfirmSeconds > 120 || s.WindowSeconds < minWindowSeconds(*s) || s.WindowSeconds > 180 || s.MinFrames < 3 || s.MinFrames > 360 || s.MinFrames > s.WindowSeconds*2 || s.MinPeople < 1 || s.MinPeople > 100 {
		return ErrInvalid
	}
	return nil
}

// 历史版本的离岗场景默认值是「60 秒确认 / 90 秒窗口 / 15 帧」，会把单路判定拖到
// 一分钟以上。这里只把完全等于历史默认元组的场景收敛到 5 秒口径；用户手动改过的
// 数值保持不动，避免覆盖现场配置。
func (s *Service) migrateAbsenceTiming() error {
	var plans []Plan
	if err := s.DB.Select("id", "spec_json").Find(&plans).Error; err != nil {
		return err
	}
	for _, plan := range plans {
		spec := plan.Spec()
		changed := false
		for ci := range spec.Cameras {
			for si := range spec.Cameras[ci].Scenes {
				scene := &spec.Cameras[ci].Scenes[si]
				if scene.Code != "post_absence" || scene.ConfirmSeconds != 60 || scene.WindowSeconds != 90 || (scene.MinFrames != 0 && scene.MinFrames != 15) {
					continue
				}
				scene.ConfirmSeconds, scene.WindowSeconds, scene.MinFrames = absenceConfirmSeconds, absenceWindowSeconds, absenceMinFrames
				changed = true
			}
		}
		if !changed {
			continue
		}
		if err := s.DB.Model(&Plan{}).Where("id = ?", plan.ID).Update("spec_json", encoded(spec)).Error; err != nil {
			return err
		}
	}
	return nil
}
func CameraTimeout(c Camera) time.Duration {
	budget := 45 * time.Second
	for _, s := range c.Scenes {
		if !IsObjectDetectionScene(s.Code) {
			_ = NormalizeScene(&s)
			if d := time.Duration(s.WindowSeconds+20) * time.Second; d > budget {
				budget = d
			}
		}
	}
	return budget
}
