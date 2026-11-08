-- ============================================================
-- StudioAttendance 数据库初始化脚本
-- 模式:没课 = 应到,按节次签到
-- 环境:MySQL 8.0+,schema: studioattendance
-- 注意:会 DROP 旧的 `user` 和 `schedules` 表(旧结构已在
--       user.sql / schedules.sql 里有备份)
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ------------------------------------------------------------
-- 1. 用户表
-- ------------------------------------------------------------
DROP TABLE IF EXISTS `user`;    -- 旧的旧单数表,由本脚本移除
DROP TABLE IF EXISTS `users`;

CREATE TABLE `users` (
  `id`         bigint       NOT NULL AUTO_INCREMENT,
  `username`   varchar(64)  NOT NULL COMMENT '登录名',
  `password`   varchar(100) NOT NULL COMMENT 'bcrypt哈希(60字符)',
  `nickname`   varchar(64)  NOT NULL COMMENT '显示昵称',
  `image`      varchar(255) NULL COMMENT '头像URL',
  `role`       tinyint      NOT NULL DEFAULT 0 COMMENT '0成员 1管理员',
  `status`     tinyint      NOT NULL DEFAULT 1 COMMENT '1在职 0禁用',
  `create_by`  bigint       NULL COMMENT '创建人',
  `update_by`  bigint       NULL COMMENT '最后修改人',
  `created_at` datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime     NULL COMMENT '软删除标记(GORM约定)',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户表';

-- ------------------------------------------------------------
-- 2. 课表表(OCR 导入,导入时先删该用户旧数据再插入)
--    判断"今天该不该上这门课":
--    weekday 匹配 && start_week <= 当前周 <= end_week && week_type 匹配
-- ------------------------------------------------------------
DROP TABLE IF EXISTS `schedules`;

CREATE TABLE `schedules` (
  `id`            bigint       NOT NULL AUTO_INCREMENT,
  `user_id`       bigint       NOT NULL COMMENT '归属用户',
  `course_name`   varchar(128) NULL COMMENT '课程名',
  `location`      varchar(128) NULL COMMENT '上课地点',
  `weekday`       tinyint      NOT NULL COMMENT '1周一 ~ 7周日',
  `start_section` int          NOT NULL COMMENT '开始节次',
  `end_section`   int          NOT NULL COMMENT '结束节次',
  `start_week`    int          NOT NULL DEFAULT 1 COMMENT '起始周(学期内)',
  `end_week`      int          NOT NULL DEFAULT 20 COMMENT '结束周(学期内)',
  `week_type`     tinyint      NOT NULL DEFAULT 0 COMMENT '0每周 1单周 2双周',
  `semester`      varchar(16)  NULL COMMENT '学期,如 2026-1',
  `created_at`    datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`    datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_user_weekday` (`user_id`, `weekday`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='课表(OCR导入)';

-- ------------------------------------------------------------
-- 3. 签到记录表(核心)
--    没课 = 应到;点击签到写 status=1;
--    每日结算定时任务补 3(缺勤)/ 5(有课豁免)
--    插入时必须显式指定 status
-- ------------------------------------------------------------
DROP TABLE IF EXISTS `checkin_records`;

CREATE TABLE `checkin_records` (
  `id`           bigint       NOT NULL AUTO_INCREMENT,
  `user_id`      bigint       NOT NULL COMMENT '用户',
  `checkin_date` date         NOT NULL COMMENT '归属日期',
  `section`      int          NOT NULL COMMENT '节次',
  `status`       tinyint      NOT NULL COMMENT '1正常 2迟到 3缺勤 4请假 5有课豁免',
  `checkin_time` datetime     NULL COMMENT '实际签到时间',
  `remark`       varchar(255) NULL,
  `created_at`   datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_date_section` (`user_id`, `checkin_date`, `section`),
  KEY `idx_date` (`checkin_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='签到记录';

SET FOREIGN_KEY_CHECKS = 1;
