-- MySQL 容器首次初始化时执行（仅当数据卷为空时生效）。
-- 集成测试使用独立的 monolith_test 库，避免污染演示数据。
CREATE DATABASE IF NOT EXISTS monolith_test;
GRANT ALL PRIVILEGES ON monolith_test.* TO 'monolith'@'%';
FLUSH PRIVILEGES;
