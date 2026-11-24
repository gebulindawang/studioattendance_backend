package repository

import (
	"StudioAttendance_backend/internal/model"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrScheduleUploadLimit  = errors.New("课表上传次数已用完")
	ErrScheduleImageChanged = errors.New("课表图片已变更，请刷新后重新解析")
)

func FindScheduleImageByID(userID int) ([]string, error) {
	urls := make([]string, 0)
	err := DB.Model(&model.ScheduleImage{}).Where("user_id = ?", userID).
		Order("id").Pluck("images_url", &urls).Error
	return urls, err
}

func FindLatestScheduleImage(userID int) (model.ScheduleImage, error) {
	var image model.ScheduleImage
	err := DB.Where("user_id = ?", userID).Order("id DESC").Take(&image).Error
	return image, err
}

func ListSchedules(userID int) ([]model.Schedule, error) {
	schedules := make([]model.Schedule, 0)
	err := DB.Where("user_id = ?", userID).Order("id").Find(&schedules).Error
	return schedules, err
}

func summarizeSchedule(user model.User, uploads []model.ScheduleImage, count int, confirmedAt *time.Time) model.ScheduleSummary {
	summary := model.ScheduleSummary{
		Key: fmt.Sprintf("u-%d", user.ID), Username: user.Username, Nickname: user.Nickname,
		Status: "none", UploadCount: len(uploads), MaxUploads: model.MaxScheduleUploads,
		CourseCount: count, ConfirmedAt: confirmedAt,
	}
	if count > 0 {
		summary.Status = "confirmed"
	}
	if len(uploads) == 0 {
		return summary
	}
	latest := uploads[len(uploads)-1]
	summary.LastUploadedAt = latest.CreatedAt
	if count == 0 {
		summary.Status = "pending"
	} else if (latest.Reviewed != nil && !*latest.Reviewed) ||
		(latest.Reviewed == nil && latest.CreatedAt != nil && confirmedAt != nil && latest.CreatedAt.After(*confirmedAt)) {
		summary.Status = "reparse"
	}
	return summary
}

func GetScheduleDetail(userID int) (model.ScheduleDetail, error) {
	detail := model.ScheduleDetail{Uploads: []model.ScheduleImage{}, Courses: []model.Schedule{}}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Select("id", "username", "nickname").First(&user, userID).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Order("id").Find(&detail.Uploads).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Order("id").Find(&detail.Courses).Error; err != nil {
			return err
		}
		var confirmedAt *time.Time
		for _, course := range detail.Courses {
			if confirmedAt == nil || course.CreatedAt.After(*confirmedAt) {
				date := course.CreatedAt
				confirmedAt = &date
			}
		}
		detail.ScheduleSummary = summarizeSchedule(user, detail.Uploads, len(detail.Courses), confirmedAt)
		if len(detail.Uploads) > 0 {
			detail.LatestImage = detail.Uploads[len(detail.Uploads)-1].ImagesUrl
		}
		return nil
	})
	return detail, err
}

func ListScheduleMembers() ([]model.ScheduleSummary, error) {
	summaries := make([]model.ScheduleSummary, 0)
	err := DB.Transaction(func(tx *gorm.DB) error {
		var members []model.User
		if err := tx.Select("id", "username", "nickname").Where("role = ?", model.RoleMember).Order("id").Find(&members).Error; err != nil {
			return err
		}

		var images []model.ScheduleImage
		if err := tx.Select("id", "user_id", "created_at", "reviewed").Order("id").Find(&images).Error; err != nil {
			return err
		}
		uploadsByUser := make(map[int][]model.ScheduleImage)
		for _, image := range images {
			uploadsByUser[image.UserID] = append(uploadsByUser[image.UserID], image)
		}

		type courseSummary struct {
			UserID      int
			Count       int
			ConfirmedAt *time.Time
		}
		var counts []courseSummary
		if err := tx.Model(&model.Schedule{}).
			Select("user_id, COUNT(*) AS count, MAX(created_at) AS confirmed_at").
			Group("user_id").Scan(&counts).Error; err != nil {
			return err
		}
		countsByUser := make(map[int]courseSummary, len(counts))
		for _, count := range counts {
			countsByUser[count.UserID] = count
		}
		for _, member := range members {
			courses := countsByUser[member.ID]
			summaries = append(summaries, summarizeSchedule(member, uploadsByUser[member.ID], courses.Count, courses.ConfirmedAt))
		}
		return nil
	})
	return summaries, err
}

func lockScheduleOwner(tx *gorm.DB, userID int) error {
	var owner model.User
	return tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).First(&owner, userID).Error
}

func ReplaceUserSchedules(userID int, schedules []model.Schedule, imageID *int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := lockScheduleOwner(tx, userID); err != nil {
			return err
		}
		var latest model.ScheduleImage
		if imageID == nil || *imageID != 0 {
			err := tx.Where("user_id = ?", userID).Order("id DESC").Take(&latest).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if imageID != nil && latest.ID != *imageID {
				return ErrScheduleImageChanged
			}
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.Schedule{}).Error; err != nil {
			return err
		}
		for i := range schedules {
			schedules[i].ID = 0
			schedules[i].UserID = userID
			schedules[i].CreatedAt = time.Time{}
			schedules[i].UpdatedAt = time.Time{}
		}
		if err := tx.Create(&schedules).Error; err != nil {
			return err
		}
		if latest.ID > 0 {
			return tx.Model(&latest).Update("reviewed", true).Error
		}
		return nil
	})
}

func UpdateSchedule(schedule model.Schedule) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var current model.Schedule
		if err := tx.Select("id", "user_id").First(&current, schedule.ID).Error; err != nil {
			return err
		}
		if err := lockScheduleOwner(tx, current.UserID); err != nil {
			return err
		}
		result := tx.Model(&model.Schedule{}).Where("id = ? AND user_id = ?", current.ID, current.UserID).
			Select("CourseName", "Location", "Weekday", "StartSection", "EndSection", "StartWeek", "EndWeek", "WeekType").
			Updates(schedule)
		if result.Error != nil || result.RowsAffected > 0 {
			return result.Error
		}
		return tx.Where("id = ? AND user_id = ?", current.ID, current.UserID).Take(&current).Error
	})
}

func CreateScheduleImage(url string, userID int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := lockScheduleOwner(tx, userID); err != nil {
			return err
		}
		count, err := countScheduleImages(tx, userID)
		if err != nil {
			return err
		}
		if count >= model.MaxScheduleUploads {
			return ErrScheduleUploadLimit
		}
		reviewed := false
		return tx.Create(&model.ScheduleImage{UserID: userID, ImagesUrl: url, Reviewed: &reviewed}).Error
	})
}

func FindImagesAmountByUserId(userID int) (int, error) {
	return countScheduleImages(DB, userID)
}

func countScheduleImages(db *gorm.DB, userID int) (int, error) {
	var count int64
	err := db.Model(&model.ScheduleImage{}).Where("user_id = ?", userID).Count(&count).Error
	return int(count), err
}

func DeleteScheduleByUserId(userID int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := lockScheduleOwner(tx, userID); err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.Schedule{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", userID).Delete(&model.ScheduleImage{}).Error
	})
}

func FindSemesterAndSectionTime() (model.SemesterConfig, error) {
	var config model.SemesterConfig
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		config, err = loadSemesterConfig(tx)
		return err
	})
	return config, err
}

func loadSemesterConfig(db *gorm.DB) (model.SemesterConfig, error) {
	config := model.SemesterConfig{SectionTime: []model.SectionTime{}}
	var semester model.Semester
	err := db.First(&semester).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return config, err
	}
	if err == nil {
		config.Semester = &semester
	}
	return config, db.Order("id").Find(&config.SectionTime).Error
}

func ValidateSchedules(schedules []model.Schedule, config model.SemesterConfig) error {
	if config.Semester == nil || len(config.SectionTime) == 0 {
		return errors.New("请先配置学期和节次")
	}
	sections := make(map[int]bool)
	for _, block := range config.SectionTime {
		if number, err := strconv.Atoi(block.SectionNumber); err == nil && number > 0 {
			sections[2*number-1], sections[2*number] = true, true
		}
	}
	for i, course := range schedules {
		if strings.TrimSpace(course.CourseName) == "" || utf8.RuneCountInString(course.CourseName) > 128 ||
			utf8.RuneCountInString(course.Location) > 128 ||
			course.Weekday < 1 || course.Weekday > 7 || course.StartSection > course.EndSection ||
			!sections[course.StartSection] || !sections[course.EndSection] ||
			course.StartWeek < 1 || course.StartWeek > course.EndWeek || course.EndWeek > config.Semester.TotalWeeks ||
			course.WeekType < model.WeekTypeEvery || course.WeekType > model.WeekTypeEven {
			return fmt.Errorf("第 %d 门课程的名称、地点、星期、节次或周次不合法", i+1)
		}
	}
	return nil
}

func UpdateSemesterOrSectionTime(config model.SemesterConfig) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var current model.Semester
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if config.Semester == nil || config.Semester.ID != 0 {
				return gorm.ErrRecordNotFound
			}
			if err := tx.Create(config.Semester).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if semester := config.Semester; semester != nil {
			if semester.ID != current.ID {
				return gorm.ErrRecordNotFound
			}
			if err := tx.Model(&current).Select("StartDate", "EndDate", "TotalWeeks").Updates(semester).Error; err != nil {
				return err
			}
		}

		for _, block := range config.SectionTime {
			if block.ID == 0 {
				if err := tx.Create(&block).Error; err != nil {
					return err
				}
				continue
			}
			var stored model.SectionTime
			if err := tx.First(&stored, block.ID).Error; err != nil {
				return err
			}
			if block.SectionNumber != stored.SectionNumber {
				return fmt.Errorf("第 %s 大节编号已变更，请刷新后重试", block.SectionNumber)
			}
			if err := tx.Model(&stored).Select("StartTime", "EndTime").Updates(block).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
