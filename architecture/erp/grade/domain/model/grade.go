// Package model 是 grade 域的领域模型。
package model

import (
	"github.com/jamieleestudio/go-tutorials/architecture/erp/shared"
)

// Grade 成绩聚合根（对应 Java 版 Grade extends AggregateRoot）。
type Grade struct {
	shared.AggregateRoot
	id         string
	studentID  string
	courseName string
	score      float64
}

// NewGrade 构造聚合 —— 业务规则：studentId/courseName 不能为空，score 0~100。
func NewGrade(id, studentID, courseName string, score float64) (*Grade, error) {
	if studentID == "" {
		return nil, shared.NewBusinessRuleViolation("studentId must not be blank")
	}
	if courseName == "" {
		return nil, shared.NewBusinessRuleViolation("courseName must not be blank")
	}
	g := &Grade{id: id, studentID: studentID, courseName: courseName, score: score}
	if err := g.validateScore(score); err != nil {
		return nil, err
	}
	return g, nil
}

// UpdateScore 改分（先校验再赋值，对应 Java 的 updateScore）。
func (g *Grade) UpdateScore(score float64) error {
	if err := g.validateScore(score); err != nil {
		return err
	}
	g.score = score
	return nil
}

// Passed 及格线 60 分（对应 Java 的 passed()）。
func (g *Grade) Passed() bool { return g.score >= 60 }

func (g *Grade) validateScore(score float64) error {
	if score < 0 || score > 100 {
		return shared.NewBusinessRuleViolation("score must be between 0 and 100")
	}
	return nil
}

func (g *Grade) ID() string         { return g.id }
func (g *Grade) StudentID() string  { return g.studentID }
func (g *Grade) CourseName() string { return g.courseName }
func (g *Grade) Score() float64     { return g.score }
