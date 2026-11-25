package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/persistence"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/system/domain/model"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/system/domain/repository"
)

// 编译期断言：实现领域仓储端口。
var _ repository.SysUserRepository = (*SysUserRepositoryImpl)(nil)

// SysUserRepositoryImpl 仓储实现（对应 Java 版 SysUserRepositoryImpl + SysUserJpaRepository）。
// Java 版的 Spring Data JPA 派生查询在这里展开为显式 gorm 调用。
type SysUserRepositoryImpl struct {
	db *gorm.DB
}

// NewSysUserRepository 构造仓储。
func NewSysUserRepository(db *gorm.DB) *SysUserRepositoryImpl {
	return &SysUserRepositoryImpl{db: db}
}

// Save 保存聚合。P2 教训复用：不用 gorm Save() 做 upsert（会把零值 created_at
// 一并写入，MySQL 上触发 Error 1292），改为按列名显式更新、0 行影响再插入。
func (r *SysUserRepositoryImpl) Save(ctx context.Context, user *model.SysUser) (*model.SysUser, error) {
	db := persistence.DBFromContext(ctx, r.db)
	id := user.ID()
	if id == "" {
		id = shared.NewID() // 对应 Java 版 IdGenerator.next()
	}
	now := time.Now()

	res := db.Model(&SysUserEntity{}).Where("id = ?", id).Updates(map[string]any{
		"name":        user.Name(),
		"is_del":      false,
		"update_time": now,
	})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		entity := SysUserEntity{
			BaseModel: persistence.BaseModel{ID: id, CreatedAt: now, UpdatedAt: now},
			Name:      user.Name(),
		}
		if err := db.Create(&entity).Error; err != nil {
			return nil, err
		}
	}
	saved, err := model.NewSysUser(id, user.Name())
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// FindByID 按主键查询；不存在返回 (nil, nil)。
func (r *SysUserRepositoryImpl) FindByID(ctx context.Context, id string) (*model.SysUser, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var entity SysUserEntity
	err := db.Where("id = ?", id).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	user, err := model.NewSysUser(entity.ID, entity.Name)
	if err != nil {
		return nil, err
	}
	return user, nil
}
