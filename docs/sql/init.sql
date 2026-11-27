-- StudioAttendance，MySQL 8.0。
-- 先创建并选中目标数据库。重复执行会保留已有记录。
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `username` varchar(64) NOT NULL,
  `password` varchar(100) NOT NULL COMMENT 'bcrypt 哈希',
  `nickname` varchar(64) NOT NULL,
  `image` varchar(255) NULL,
  `role` tinyint NOT NULL DEFAULT 0 COMMENT '0 成员，1 管理员',
  `status` tinyint NOT NULL DEFAULT 1 COMMENT '1 在职，0 禁用',
  `create_by` bigint NULL,
  `update_by` bigint NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `schedules` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` bigint NOT NULL,
  `course_name` varchar(128) NULL,
  `location` varchar(128) NULL,
  `weekday` tinyint NOT NULL COMMENT '1 周一至 7 周日',
  `start_section` int NOT NULL,
  `end_section` int NOT NULL,
  `start_week` int NOT NULL DEFAULT 1,
  `end_week` int NOT NULL DEFAULT 20,
  `week_type` tinyint NOT NULL DEFAULT 0 COMMENT '0 每周，1 单周，2 双周',
  `semester` varchar(16) NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_user_weekday` (`user_id`, `weekday`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 工作室签到按大节生成。旧版小节记录的 record_kind 为 0。
CREATE TABLE IF NOT EXISTS `checkin_records` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` bigint NOT NULL,
  `checkin_date` date NOT NULL,
  `section` int NOT NULL,
  `status` tinyint NOT NULL COMMENT '0 待签到，1 已签到，2 至 5 为历史状态，7 已补签',
  `checkin_time` datetime NULL,
  `remark` varchar(255) NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `record_kind` tinyint NOT NULL DEFAULT 0 COMMENT '0 历史小节考勤，1 工作室大节签到',
  `slot_start` datetime NULL,
  `slot_end` datetime NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_date_section_kind` (`user_id`, `checkin_date`, `section`, `record_kind`),
  KEY `idx_date` (`checkin_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `schedules_image` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` bigint NOT NULL,
  `images_url` varchar(2048) NOT NULL,
  `created_at` datetime(6) NULL,
  `reviewed` boolean NULL,
  PRIMARY KEY (`id`),
  KEY `idx_schedule_image_user` (`user_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `checkin_makeup_requests` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `record_id` bigint NOT NULL,
  `reason` varchar(500) NOT NULL,
  `status` varchar(16) NOT NULL DEFAULT 'pending',
  `reviewer_id` bigint NULL,
  `review_note` varchar(500) NOT NULL DEFAULT '',
  `reviewed_at` datetime NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_makeup_record` (`record_id`, `id`),
  KEY `idx_makeup_status` (`status`, `id`),
  CONSTRAINT `fk_makeup_checkin_record` FOREIGN KEY (`record_id`) REFERENCES `checkin_records` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `semester` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `start_date` date NOT NULL,
  `end_date` date NOT NULL,
  `total_weeks` int NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 第 N 大节对应第 2N-1、2N 小节。
CREATE TABLE IF NOT EXISTS `section_time` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `section_number` varchar(16) NOT NULL,
  `start_time` time NOT NULL,
  `end_time` time NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_section_number` (`section_number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 补齐已部署版本缺少的列。没有记录的历史上传时间和审核状态保留 NULL。
SET @ddl = IF(
  EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE()
    AND table_name = 'schedules_image' AND column_name = 'created_at'),
  'DO 0', 'ALTER TABLE schedules_image ADD COLUMN created_at datetime(6) NULL'
);
PREPARE studio_attendance_ddl FROM @ddl;
EXECUTE studio_attendance_ddl;
DEALLOCATE PREPARE studio_attendance_ddl;

-- 增加工作室大节字段并替换旧唯一键，让历史小节与新大节可以共存。
SET @checkin_alter = CONCAT_WS(', ',
  IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE()
    AND table_name = 'checkin_records' AND column_name = 'record_kind'),
    NULL, 'ADD COLUMN record_kind tinyint NOT NULL DEFAULT 0'),
  IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE()
    AND table_name = 'checkin_records' AND column_name = 'slot_start'),
    NULL, 'ADD COLUMN slot_start datetime NULL'),
  IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE()
    AND table_name = 'checkin_records' AND column_name = 'slot_end'),
    NULL, 'ADD COLUMN slot_end datetime NULL'),
  IF(EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE()
    AND table_name = 'checkin_records' AND index_name = 'uk_user_date_section_kind'),
    NULL, 'ADD UNIQUE KEY uk_user_date_section_kind (user_id, checkin_date, section, record_kind)'),
  IF(EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE()
    AND table_name = 'checkin_records' AND index_name = 'uk_user_date_section'),
    'DROP INDEX uk_user_date_section', NULL)
);
SET @ddl = IF(@checkin_alter = '', 'DO 0', CONCAT('ALTER TABLE checkin_records ', @checkin_alter));
PREPARE studio_attendance_ddl FROM @ddl;
EXECUTE studio_attendance_ddl;
DEALLOCATE PREPARE studio_attendance_ddl;

SET @ddl = IF(
  EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE()
    AND table_name = 'schedules_image' AND column_name = 'reviewed'),
  'DO 0', 'ALTER TABLE schedules_image ADD COLUMN reviewed boolean NULL'
);
PREPARE studio_attendance_ddl FROM @ddl;
EXECUTE studio_attendance_ddl;
DEALLOCATE PREPARE studio_attendance_ddl;
