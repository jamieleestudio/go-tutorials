package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/grade/domain/model"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/grade/domain/repository"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/persistence"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/shared"
)

// 编译期断言：实现领域仓储端口。
var _ repository.GradeRepository = (*GradeRepositoryImpl)(nil)

// GradeRepositoryImpl 成绩仓储实现（对应 Java 版 GradeRepositoryImpl）。
type GradeRepositoryImpl struct {
	db *gorm.DB
}

// NewGradeRepository 构造仓储。
func NewGradeRepository(db *gorm.DB) *GradeRepositoryImpl {
	return &GradeRepositoryImpl{db: db}
}

// Save 保存聚合（按列名显式更新、0 行再插入 —— P2 教训，见 system 域同款注释）。
func (r *GradeRepositoryImpl) Save(ctx context.Context, grade *model.Grade) (*model.Grade, error) {
	db := persistence.DBFromContext(ctx, r.db)
	id := grade.ID()
	if id == "" {
		id = shared.NewID()
	}
	now := time.Now()

	res := db.Model(&GradeEntity{}).Where("id = ?", id).Updates(map[string]any{
		"student_id":  grade.StudentID(),
		"course_name": grade.CourseName(),
		"score":       grade.Score(),
		"is_del":      false,
		"update_time": now,
	})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		entity := GradeEntity{
			BaseModel:  persistence.BaseModel{ID: id, CreatedAt: now, UpdatedAt: now},
			StudentID:  grade.StudentID(),
			CourseName: grade.CourseName(),
			Score:      grade.Score(),
		}
		if err := db.Create(&entity).Error; err != nil {
			return nil, err
		}
	}
	return model.NewGrade(id, grade.StudentID(), grade.CourseName(), grade.Score())
}

// FindByID 按主键查询；不存在返回 (nil, nil)。
func (r *GradeRepositoryImpl) FindByID(ctx context.Context, id string) (*model.Grade, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entity GradeEntity
	err := db.Where("id = ?", id).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return model.NewGrade(entity.ID, entity.StudentID, entity.CourseName, entity.Score)
}

// FindByStudentID 按学生查询（对应 Java 的 findByStudentId 派生查询）。
func (r *GradeRepositoryImpl) FindByStudentID(ctx context.Context, studentID string) ([]*model.Grade, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entities []GradeEntity
	if err := db.Where("student_id = ?", studentID).Find(&entities).Error; err != nil {
		return nil, err
	}
	grades := make([]*model.Grade, 0, len(entities))
	for _, e := range entities {
		grade, err := model.NewGrade(e.ID, e.StudentID, e.CourseName, e.Score)
		if err != nil {
			return nil, err
		}
		grades = append(grades, grade)
	}
	return grades, nil
}
