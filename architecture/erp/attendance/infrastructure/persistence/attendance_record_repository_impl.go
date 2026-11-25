package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/domain/model"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/domain/repository"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/persistence"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/shared"
)

// 编译期断言：实现领域仓储端口。
var _ repository.AttendanceRecordRepository = (*AttendanceRecordRepositoryImpl)(nil)

// AttendanceRecordRepositoryImpl 考勤记录仓储实现。
type AttendanceRecordRepositoryImpl struct {
	db *gorm.DB
}

// NewAttendanceRecordRepository 构造仓储。
func NewAttendanceRecordRepository(db *gorm.DB) *AttendanceRecordRepositoryImpl {
	return &AttendanceRecordRepositoryImpl{db: db}
}

// Save 保存聚合（按列名显式更新、0 行再插入 —— P2 教训）。
func (r *AttendanceRecordRepositoryImpl) Save(ctx context.Context, record *model.AttendanceRecord) (*model.AttendanceRecord, error) {
	db := persistence.DBFromContext(ctx, r.db)
	id := record.ID()
	if id == "" {
		id = shared.NewID()
	}
	now := time.Now()

	res := db.Model(&AttendanceRecordEntity{}).Where("id = ?", id).Updates(map[string]any{
		"student_id":    record.StudentID(),
		"status":        string(record.Status()),
		"clock_in_time": record.ClockInTime(),
		"is_del":        false,
		"update_time":   now,
	})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		entity := AttendanceRecordEntity{
			BaseModel:   persistence.BaseModel{ID: id, CreatedAt: now, UpdatedAt: now},
			StudentID:   record.StudentID(),
			Status:      string(record.Status()),
			ClockInTime: record.ClockInTime(),
		}
		if err := db.Create(&entity).Error; err != nil {
			return nil, err
		}
	}
	return model.NewAttendanceRecord(id, record.StudentID(), record.Status(), record.ClockInTime()), nil
}

// FindByID 按主键查询；不存在返回 (nil, nil)。
func (r *AttendanceRecordRepositoryImpl) FindByID(ctx context.Context, id string) (*model.AttendanceRecord, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entity AttendanceRecordEntity
	err := db.Where("id = ?", id).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomain(entity), nil
}

// FindByStudentID 按学生查询。
func (r *AttendanceRecordRepositoryImpl) FindByStudentID(ctx context.Context, studentID string) ([]*model.AttendanceRecord, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entities []AttendanceRecordEntity
	if err := db.Where("student_id = ?", studentID).Find(&entities).Error; err != nil {
		return nil, err
	}
	records := make([]*model.AttendanceRecord, 0, len(entities))
	for _, e := range entities {
		records = append(records, toDomain(e))
	}
	return records, nil
}

func toDomain(e AttendanceRecordEntity) *model.AttendanceRecord {
	return model.NewAttendanceRecord(e.ID, e.StudentID, model.AttendanceStatus(e.Status), e.ClockInTime)
}
