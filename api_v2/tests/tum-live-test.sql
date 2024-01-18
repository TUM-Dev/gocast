CREATE DATABASE IF NOT EXISTS `tumlive_test`;
USE `tumlive_test`;


-- MariaDB dump 10.19-11.2.2-MariaDB, for Linux (x86_64)
--
-- Host: 127.0.0.1    Database: tumlive_test
-- ------------------------------------------------------
-- Server version	5.5.5-10.7.3-MariaDB-1:10.7.3+maria~focal

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `audits`
--

DROP TABLE IF EXISTS `audits`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `audits` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` bigint(20) unsigned DEFAULT NULL,
  `message` longtext DEFAULT NULL,
  `type` bigint(20) unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_audits_deleted_at` (`deleted_at`),
  KEY `fk_audits_user` (`user_id`),
  CONSTRAINT `fk_audits_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `audits`
--

LOCK TABLES `audits` WRITE;
/*!40000 ALTER TABLE `audits` DISABLE KEYS */;
/*!40000 ALTER TABLE `audits` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `bookmarks`
--

DROP TABLE IF EXISTS `bookmarks`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `bookmarks` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `description` longtext NOT NULL,
  `hours` bigint(20) unsigned NOT NULL,
  `minutes` bigint(20) unsigned NOT NULL,
  `seconds` bigint(20) unsigned NOT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  `stream_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_bookmarks_deleted_at` (`deleted_at`),
  KEY `fk_users_bookmarks` (`user_id`),
  CONSTRAINT `fk_users_bookmarks` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=10 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `bookmarks`
--

LOCK TABLES `bookmarks` WRITE;
/*!40000 ALTER TABLE `bookmarks` DISABLE KEYS */;
/*!40000 ALTER TABLE `bookmarks` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `camera_presets`
--

DROP TABLE IF EXISTS `camera_presets`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `camera_presets` (
  `name` longtext NOT NULL,
  `preset_id` bigint(20) NOT NULL,
  `image` longtext DEFAULT NULL,
  `lecture_hall_id` bigint(20) unsigned NOT NULL,
  `default` tinyint(1) DEFAULT NULL,
  `is_default` tinyint(1) DEFAULT NULL,
  PRIMARY KEY (`preset_id`,`lecture_hall_id`),
  KEY `fk_lecture_halls_camera_presets` (`lecture_hall_id`),
  CONSTRAINT `fk_lecture_halls_camera_presets` FOREIGN KEY (`lecture_hall_id`) REFERENCES `lecture_halls` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `camera_presets`
--

LOCK TABLES `camera_presets` WRITE;
/*!40000 ALTER TABLE `camera_presets` DISABLE KEYS */;
/*!40000 ALTER TABLE `camera_presets` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `chat_poll_options`
--

DROP TABLE IF EXISTS `chat_poll_options`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `chat_poll_options` (
  `poll_id` bigint(20) unsigned NOT NULL,
  `poll_option_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`poll_id`,`poll_option_id`),
  KEY `fk_chat_poll_options_poll_option` (`poll_option_id`),
  CONSTRAINT `fk_chat_poll_options_poll` FOREIGN KEY (`poll_id`) REFERENCES `polls` (`id`),
  CONSTRAINT `fk_chat_poll_options_poll_option` FOREIGN KEY (`poll_option_id`) REFERENCES `poll_options` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `chat_poll_options`
--

LOCK TABLES `chat_poll_options` WRITE;
/*!40000 ALTER TABLE `chat_poll_options` DISABLE KEYS */;
/*!40000 ALTER TABLE `chat_poll_options` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `chat_reactions`
--

DROP TABLE IF EXISTS `chat_reactions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `chat_reactions` (
  `chat_id` bigint(20) unsigned NOT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  `username` longtext NOT NULL,
  `emoji` varchar(191) NOT NULL,
  PRIMARY KEY (`chat_id`,`user_id`,`emoji`),
  CONSTRAINT `fk_chats_reactions` FOREIGN KEY (`chat_id`) REFERENCES `chats` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `chat_reactions`
--

LOCK TABLES `chat_reactions` WRITE;
/*!40000 ALTER TABLE `chat_reactions` DISABLE KEYS */;
/*!40000 ALTER TABLE `chat_reactions` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `chat_user_addressedto`
--

DROP TABLE IF EXISTS `chat_user_addressedto`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `chat_user_addressedto` (
  `chat_id` bigint(20) unsigned NOT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`chat_id`,`user_id`),
  KEY `fk_chat_user_addressedto_user` (`user_id`),
  CONSTRAINT `fk_chat_user_addressedto_chat` FOREIGN KEY (`chat_id`) REFERENCES `chats` (`id`),
  CONSTRAINT `fk_chat_user_addressedto_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `chat_user_addressedto`
--

LOCK TABLES `chat_user_addressedto` WRITE;
/*!40000 ALTER TABLE `chat_user_addressedto` DISABLE KEYS */;
/*!40000 ALTER TABLE `chat_user_addressedto` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `chats`
--

DROP TABLE IF EXISTS `chats`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `chats` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` longtext NOT NULL,
  `user_name` longtext NOT NULL,
  `message` text NOT NULL,
  `stream_id` bigint(20) unsigned NOT NULL,
  `admin` tinyint(1) NOT NULL DEFAULT 0,
  `color` varchar(191) NOT NULL DEFAULT '#368bd6',
  `visible` tinyint(1) NOT NULL DEFAULT 1,
  `reply_to` bigint(20) unsigned DEFAULT NULL,
  `resolved` tinyint(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_chats_deleted_at` (`deleted_at`),
  KEY `fk_chats_replies` (`reply_to`),
  KEY `fk_streams_chats` (`stream_id`),
  FULLTEXT KEY `idx_chats_message` (`message`),
  CONSTRAINT `fk_chats_replies` FOREIGN KEY (`reply_to`) REFERENCES `chats` (`id`),
  CONSTRAINT `fk_streams_chats` FOREIGN KEY (`stream_id`) REFERENCES `streams` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `chats`
--

LOCK TABLES `chats` WRITE;
/*!40000 ALTER TABLE `chats` DISABLE KEYS */;
/*!40000 ALTER TABLE `chats` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `course_admins`
--

DROP TABLE IF EXISTS `course_admins`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `course_admins` (
  `course_id` bigint(20) unsigned NOT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`course_id`,`user_id`),
  KEY `fk_course_admins_user` (`user_id`),
  CONSTRAINT `fk_course_admins_course` FOREIGN KEY (`course_id`) REFERENCES `courses` (`id`),
  CONSTRAINT `fk_course_admins_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `course_admins`
--

LOCK TABLES `course_admins` WRITE;
/*!40000 ALTER TABLE `course_admins` DISABLE KEYS */;
INSERT INTO `course_admins` VALUES
(1,2),
(3,2),
(1,3),
(2,3);
/*!40000 ALTER TABLE `course_admins` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `course_users`
--

DROP TABLE IF EXISTS `course_users`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `course_users` (
  `course_id` bigint(20) unsigned NOT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`course_id`,`user_id`),
  KEY `fk_course_users_user` (`user_id`),
  CONSTRAINT `fk_course_users_course` FOREIGN KEY (`course_id`) REFERENCES `courses` (`id`),
  CONSTRAINT `fk_course_users_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `course_users`
--

LOCK TABLES `course_users` WRITE;
/*!40000 ALTER TABLE `course_users` DISABLE KEYS */;
INSERT INTO `course_users` VALUES
(1,4),
(2,4),
(1,5),
(2,5),
(1,6),
(3,6),
(1,8),
(2,8),
(3,8),
(4,8),
(5,8);
/*!40000 ALTER TABLE `course_users` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `courses`
--

DROP TABLE IF EXISTS `courses`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `courses` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  `name` longtext NOT NULL,
  `slug` longtext NOT NULL,
  `year` bigint(20) NOT NULL,
  `teaching_term` longtext NOT NULL,
  `tum_online_identifier` longtext DEFAULT NULL,
  `vod_enabled` tinyint(1) DEFAULT 1,
  `downloads_enabled` tinyint(1) DEFAULT 0,
  `chat_enabled` tinyint(1) DEFAULT 0,
  `anonymous_chat_enabled` tinyint(1) NOT NULL DEFAULT 1,
  `moderated_chat_enabled` tinyint(1) NOT NULL DEFAULT 0,
  `vod_chat_enabled` tinyint(1) DEFAULT NULL,
  `visibility` varchar(191) DEFAULT 'loggedin',
  `token` longtext DEFAULT NULL,
  `user_created_by_token` tinyint(1) DEFAULT 0,
  `camera_preset_preferences` longtext DEFAULT NULL,
  `source_preferences` longtext DEFAULT NULL,
  `live_private` tinyint(1) NOT NULL DEFAULT 0,
  `vod_private` tinyint(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_courses_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `courses`
--

LOCK TABLES `courses` WRITE;
/*!40000 ALTER TABLE `courses` DISABLE KEYS */;
INSERT INTO `courses` VALUES
(1,'2022-04-18 13:40:05.843','2022-04-18 13:46:46.546',NULL,1,'Course: public','pub',2022,'S','',1,1,0,1,0,0,'public','',0,'',NULL,0,0),
(2,'2022-04-18 13:40:54.686','2022-04-18 13:40:54.698',NULL,1,'Course: loggedin','log',2022,'S','',1,1,0,1,0,0,'loggedin','',0,'',NULL,0,0),
(3,'2022-04-18 13:41:55.741','2022-04-18 13:41:55.754',NULL,1,'Course: enrolled','enr',2021,'W','',1,1,0,1,0,0,'enrolled','',0,'',NULL,0,0),
(4,'2022-04-18 13:41:55.741','2022-04-18 13:41:55.754',NULL,1,'Course: private','priv',2021,'W','',1,1,0,1,0,0,'private','',0,'',NULL,0,0),
(5,'2022-04-18 13:41:55.741','2022-04-18 13:41:55.754',NULL,1,'Course without streams','nostream',2021,'W','',1,1,0,1,0,0,'public','',0,'',NULL,0,0);
/*!40000 ALTER TABLE `courses` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `devices`
--

DROP TABLE IF EXISTS `devices`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `devices` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` bigint(20) unsigned DEFAULT NULL,
  `device_token` varchar(256) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_devices_deleted_at` (`deleted_at`),
  KEY `fk_devices_user` (`user_id`),
  CONSTRAINT `fk_devices_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `devices`
--

LOCK TABLES `devices` WRITE;
/*!40000 ALTER TABLE `devices` DISABLE KEYS */;
/*!40000 ALTER TABLE `devices` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `emails`
--

DROP TABLE IF EXISTS `emails`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `emails` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `from` longtext NOT NULL,
  `to` longtext NOT NULL,
  `subject` longtext NOT NULL,
  `body` longtext NOT NULL,
  `success` tinyint(1) NOT NULL DEFAULT 0,
  `retries` bigint(20) NOT NULL DEFAULT 0,
  `last_try` datetime(3) DEFAULT NULL,
  `errors` varchar(191) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_emails_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `emails`
--

LOCK TABLES `emails` WRITE;
/*!40000 ALTER TABLE `emails` DISABLE KEYS */;
/*!40000 ALTER TABLE `emails` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `files`
--

DROP TABLE IF EXISTS `files`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `files` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `stream_id` bigint(20) unsigned NOT NULL,
  `path` longtext NOT NULL,
  `filename` longtext DEFAULT NULL,
  `type` bigint(20) unsigned NOT NULL DEFAULT 1,
  PRIMARY KEY (`id`),
  KEY `idx_files_deleted_at` (`deleted_at`),
  KEY `fk_streams_files` (`stream_id`),
  CONSTRAINT `fk_streams_files` FOREIGN KEY (`stream_id`) REFERENCES `streams` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `files`
--

LOCK TABLES `files` WRITE;
/*!40000 ALTER TABLE `files` DISABLE KEYS */;
/*!40000 ALTER TABLE `files` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `info_pages`
--

DROP TABLE IF EXISTS `info_pages`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `info_pages` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `name` longtext NOT NULL,
  `raw_content` longtext NOT NULL,
  `type` bigint(20) unsigned NOT NULL DEFAULT 1,
  PRIMARY KEY (`id`),
  KEY `idx_info_pages_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `info_pages`
--

LOCK TABLES `info_pages` WRITE;
/*!40000 ALTER TABLE `info_pages` DISABLE KEYS */;
/*!40000 ALTER TABLE `info_pages` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `ingest_servers`
--

DROP TABLE IF EXISTS `ingest_servers`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `ingest_servers` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `url` longtext DEFAULT NULL,
  `out_url` longtext NOT NULL,
  `workload` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_ingest_servers_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ingest_servers`
--

LOCK TABLES `ingest_servers` WRITE;
/*!40000 ALTER TABLE `ingest_servers` DISABLE KEYS */;
/*!40000 ALTER TABLE `ingest_servers` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `lecture_halls`
--

DROP TABLE IF EXISTS `lecture_halls`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `lecture_halls` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `name` longtext NOT NULL,
  `full_name` longtext NOT NULL,
  `comb_ip` longtext DEFAULT NULL,
  `pres_ip` longtext DEFAULT NULL,
  `cam_ip` longtext DEFAULT NULL,
  `camera_ip` longtext DEFAULT NULL,
  `room_id` bigint(20) DEFAULT NULL,
  `pwr_ctrl_ip` longtext DEFAULT NULL,
  `live_light_index` bigint(20) DEFAULT NULL,
  `camera_type` bigint(20) unsigned NOT NULL DEFAULT 1,
  `external_url` longtext DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_lecture_halls_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `lecture_halls`
--

LOCK TABLES `lecture_halls` WRITE;
/*!40000 ALTER TABLE `lecture_halls` DISABLE KEYS */;
/*!40000 ALTER TABLE `lecture_halls` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `migrations`
--

DROP TABLE IF EXISTS `migrations`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `migrations` (
  `id` varchar(255) NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `migrations`
--

LOCK TABLES `migrations` WRITE;
/*!40000 ALTER TABLE `migrations` DISABLE KEYS */;
INSERT INTO `migrations` VALUES
('202201280'),
('202207240'),
('202208110'),
('202210080'),
('202210270'),
('202212010'),
('202212020'),
('202301006');
/*!40000 ALTER TABLE `migrations` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `notifications`
--

DROP TABLE IF EXISTS `notifications`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `notifications` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `title` longtext DEFAULT NULL,
  `body` longtext NOT NULL,
  `target` bigint(20) NOT NULL DEFAULT 1,
  PRIMARY KEY (`id`),
  KEY `idx_notifications_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `notifications`
--

LOCK TABLES `notifications` WRITE;
/*!40000 ALTER TABLE `notifications` DISABLE KEYS */;
/*!40000 ALTER TABLE `notifications` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `pinned_courses`
--

DROP TABLE IF EXISTS `pinned_courses`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `pinned_courses` (
  `user_id` bigint(20) unsigned NOT NULL,
  `course_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`user_id`,`course_id`),
  KEY `fk_pinned_courses_course` (`course_id`),
  CONSTRAINT `fk_pinned_courses_course` FOREIGN KEY (`course_id`) REFERENCES `courses` (`id`),
  CONSTRAINT `fk_pinned_courses_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `pinned_courses`
--

LOCK TABLES `pinned_courses` WRITE;
/*!40000 ALTER TABLE `pinned_courses` DISABLE KEYS */;
/*!40000 ALTER TABLE `pinned_courses` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `poll_option_user_votes`
--

DROP TABLE IF EXISTS `poll_option_user_votes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `poll_option_user_votes` (
  `poll_option_id` bigint(20) unsigned NOT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`poll_option_id`,`user_id`),
  KEY `fk_poll_option_user_votes_user` (`user_id`),
  CONSTRAINT `fk_poll_option_user_votes_poll_option` FOREIGN KEY (`poll_option_id`) REFERENCES `poll_options` (`id`),
  CONSTRAINT `fk_poll_option_user_votes_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `poll_option_user_votes`
--

LOCK TABLES `poll_option_user_votes` WRITE;
/*!40000 ALTER TABLE `poll_option_user_votes` DISABLE KEYS */;
/*!40000 ALTER TABLE `poll_option_user_votes` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `poll_options`
--

DROP TABLE IF EXISTS `poll_options`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `poll_options` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `answer` longtext NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_poll_options_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `poll_options`
--

LOCK TABLES `poll_options` WRITE;
/*!40000 ALTER TABLE `poll_options` DISABLE KEYS */;
/*!40000 ALTER TABLE `poll_options` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `polls`
--

DROP TABLE IF EXISTS `polls`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `polls` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `stream_id` bigint(20) unsigned DEFAULT NULL,
  `question` longtext NOT NULL,
  `active` tinyint(1) NOT NULL DEFAULT 1,
  PRIMARY KEY (`id`),
  KEY `idx_polls_deleted_at` (`deleted_at`),
  KEY `fk_polls_stream` (`stream_id`),
  CONSTRAINT `fk_polls_stream` FOREIGN KEY (`stream_id`) REFERENCES `streams` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `polls`
--

LOCK TABLES `polls` WRITE;
/*!40000 ALTER TABLE `polls` DISABLE KEYS */;
/*!40000 ALTER TABLE `polls` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `register_links`
--

DROP TABLE IF EXISTS `register_links`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `register_links` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  `register_secret` longtext NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_register_links_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `register_links`
--

LOCK TABLES `register_links` WRITE;
/*!40000 ALTER TABLE `register_links` DISABLE KEYS */;
/*!40000 ALTER TABLE `register_links` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `server_notifications`
--

DROP TABLE IF EXISTS `server_notifications`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `server_notifications` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `text` longtext NOT NULL,
  `warn` tinyint(1) NOT NULL DEFAULT 0,
  `start` datetime(3) NOT NULL,
  `expires` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_server_notifications_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `server_notifications`
--

LOCK TABLES `server_notifications` WRITE;
/*!40000 ALTER TABLE `server_notifications` DISABLE KEYS */;
/*!40000 ALTER TABLE `server_notifications` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `short_links`
--

DROP TABLE IF EXISTS `short_links`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `short_links` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `link` varchar(256) NOT NULL,
  `course_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `link` (`link`),
  KEY `idx_short_links_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `short_links`
--

LOCK TABLES `short_links` WRITE;
/*!40000 ALTER TABLE `short_links` DISABLE KEYS */;
/*!40000 ALTER TABLE `short_links` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `silences`
--

DROP TABLE IF EXISTS `silences`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `silences` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `start` bigint(20) unsigned DEFAULT NULL,
  `end` bigint(20) unsigned DEFAULT NULL,
  `stream_id` bigint(20) unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_silences_deleted_at` (`deleted_at`),
  KEY `fk_streams_silences` (`stream_id`),
  CONSTRAINT `fk_streams_silences` FOREIGN KEY (`stream_id`) REFERENCES `streams` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `silences`
--

LOCK TABLES `silences` WRITE;
/*!40000 ALTER TABLE `silences` DISABLE KEYS */;
/*!40000 ALTER TABLE `silences` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `stats`
--

DROP TABLE IF EXISTS `stats`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `stats` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `time` datetime(3) NOT NULL,
  `stream_id` bigint(20) unsigned NOT NULL,
  `viewers` bigint(20) unsigned NOT NULL DEFAULT 0,
  `live` tinyint(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_stats_deleted_at` (`deleted_at`),
  KEY `fk_streams_stats` (`stream_id`),
  CONSTRAINT `fk_streams_stats` FOREIGN KEY (`stream_id`) REFERENCES `streams` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=10 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `stats`
--

LOCK TABLES `stats` WRITE;
/*!40000 ALTER TABLE `stats` DISABLE KEYS */;
INSERT INTO `stats` VALUES
(1,'2022-04-18 13:49:00.002','2022-04-18 13:49:00.002',NULL,'2022-04-18 13:49:00.001',7,0,1),
(2,'2022-04-18 13:50:00.002','2022-04-18 13:50:00.002',NULL,'2022-04-18 13:50:00.001',7,0,1),
(3,'2022-04-18 13:51:00.001','2022-04-18 13:51:00.001',NULL,'2022-04-18 13:51:00.000',7,0,1),
(4,'2022-04-18 13:52:00.002','2022-04-18 13:52:00.002',NULL,'2022-04-18 13:52:00.001',7,0,1),
(5,'2022-04-18 13:53:00.002','2022-04-18 13:53:00.002',NULL,'2022-04-18 13:53:00.001',7,0,1),
(6,'2022-04-18 13:54:00.002','2022-04-18 13:54:00.002',NULL,'2022-04-18 13:54:00.001',7,0,1),
(7,'2022-04-18 13:55:00.002','2022-04-18 13:55:00.002',NULL,'2022-04-18 13:55:00.001',7,0,1),
(8,'2022-04-18 13:56:00.002','2022-04-18 13:56:00.002',NULL,'2022-04-18 13:56:00.001',7,0,1),
(9,'2022-04-18 13:57:00.002','2022-04-18 13:57:00.002',NULL,'2022-04-18 13:57:00.000',7,0,1);
/*!40000 ALTER TABLE `stats` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `stream_names`
--

DROP TABLE IF EXISTS `stream_names`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `stream_names` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `stream_name` varchar(64) NOT NULL,
  `is_transcoding` tinyint(1) NOT NULL DEFAULT 0,
  `ingest_server_id` bigint(20) unsigned NOT NULL,
  `stream_id` bigint(20) unsigned DEFAULT NULL,
  `freed_at` datetime(3) NOT NULL DEFAULT '2024-01-03 00:00:00.000',
  PRIMARY KEY (`id`),
  UNIQUE KEY `stream_name` (`stream_name`),
  KEY `idx_stream_names_deleted_at` (`deleted_at`),
  KEY `fk_ingest_servers_stream_names` (`ingest_server_id`),
  CONSTRAINT `fk_ingest_servers_stream_names` FOREIGN KEY (`ingest_server_id`) REFERENCES `ingest_servers` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `stream_names`
--

LOCK TABLES `stream_names` WRITE;
/*!40000 ALTER TABLE `stream_names` DISABLE KEYS */;
/*!40000 ALTER TABLE `stream_names` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `stream_progresses`
--

DROP TABLE IF EXISTS `stream_progresses`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `stream_progresses` (
  `progress` double NOT NULL,
  `watched` tinyint(1) NOT NULL DEFAULT 0,
  `stream_id` bigint(20) unsigned NOT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`stream_id`,`user_id`),
  CONSTRAINT `fk_streams_stream_progresses` FOREIGN KEY (`stream_id`) REFERENCES `streams` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `stream_progresses`
--

LOCK TABLES `stream_progresses` WRITE;
/*!40000 ALTER TABLE `stream_progresses` DISABLE KEYS */;
INSERT INTO `stream_progresses` VALUES
(0,0,1,1);
/*!40000 ALTER TABLE `stream_progresses` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `stream_units`
--

DROP TABLE IF EXISTS `stream_units`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `stream_units` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `unit_name` longtext DEFAULT NULL,
  `unit_description` longtext DEFAULT NULL,
  `unit_start` bigint(20) unsigned NOT NULL,
  `unit_end` bigint(20) unsigned NOT NULL,
  `stream_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_stream_units_deleted_at` (`deleted_at`),
  KEY `fk_streams_units` (`stream_id`),
  CONSTRAINT `fk_streams_units` FOREIGN KEY (`stream_id`) REFERENCES `streams` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `stream_units`
--

LOCK TABLES `stream_units` WRITE;
/*!40000 ALTER TABLE `stream_units` DISABLE KEYS */;
/*!40000 ALTER TABLE `stream_units` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `stream_workers`
--

DROP TABLE IF EXISTS `stream_workers`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `stream_workers` (
  `stream_id` bigint(20) unsigned NOT NULL,
  `worker_worker_id` varchar(191) NOT NULL,
  PRIMARY KEY (`stream_id`,`worker_worker_id`),
  KEY `fk_stream_workers_worker` (`worker_worker_id`),
  CONSTRAINT `fk_stream_workers_stream` FOREIGN KEY (`stream_id`) REFERENCES `streams` (`id`),
  CONSTRAINT `fk_stream_workers_worker` FOREIGN KEY (`worker_worker_id`) REFERENCES `workers` (`worker_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `stream_workers`
--

LOCK TABLES `stream_workers` WRITE;
/*!40000 ALTER TABLE `stream_workers` DISABLE KEYS */;
/*!40000 ALTER TABLE `stream_workers` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `streams`
--

DROP TABLE IF EXISTS `streams`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `streams` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `name` varchar(191) DEFAULT NULL,
  `description` text DEFAULT NULL,
  `course_id` bigint(20) unsigned DEFAULT NULL,
  `start` datetime(3) NOT NULL,
  `end` datetime(3) NOT NULL,
  `room_name` longtext DEFAULT NULL,
  `room_code` longtext DEFAULT NULL,
  `event_type_name` longtext DEFAULT NULL,
  `tum_online_event_id` bigint(20) unsigned DEFAULT NULL,
  `series_identifier` varchar(191) DEFAULT NULL,
  `stream_key` longtext NOT NULL,
  `playlist_url` longtext DEFAULT NULL,
  `playlist_url_pres` longtext DEFAULT NULL,
  `playlist_url_cam` longtext DEFAULT NULL,
  `file_path` longtext DEFAULT NULL,
  `live_now` tinyint(1) NOT NULL,
  `recording` tinyint(1) DEFAULT NULL,
  `premiere` tinyint(1) DEFAULT NULL,
  `ended` tinyint(1) DEFAULT NULL,
  `vod_views` bigint(20) unsigned DEFAULT 0,
  `start_offset` bigint(20) unsigned DEFAULT NULL,
  `end_offset` bigint(20) unsigned DEFAULT NULL,
  `lecture_hall_id` bigint(20) unsigned DEFAULT NULL,
  `stream_name` longtext DEFAULT NULL,
  `duration` int(10) unsigned DEFAULT NULL,
  `chat_enabled` tinyint(1) DEFAULT NULL,
  `live_now_timestamp` datetime(3) DEFAULT NULL,
  `thumb_interval` int(10) unsigned DEFAULT NULL,
  `private` tinyint(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_streams_deleted_at` (`deleted_at`),
  KEY `fk_courses_streams` (`course_id`),
  KEY `fk_lecture_halls_streams` (`lecture_hall_id`),
  FULLTEXT KEY `idx_streams_name` (`name`),
  FULLTEXT KEY `idx_streams_description` (`description`),
  CONSTRAINT `fk_courses_streams` FOREIGN KEY (`course_id`) REFERENCES `courses` (`id`),
  CONSTRAINT `fk_lecture_halls_streams` FOREIGN KEY (`lecture_hall_id`) REFERENCES `lecture_halls` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=10 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `streams`
--

LOCK TABLES `streams` WRITE;
/*!40000 ALTER TABLE `streams` DISABLE KEYS */;
INSERT INTO `streams` VALUES
(1,'2022-04-18 13:45:58.657','2022-04-18 13:46:46.547',NULL,'VL 1: Was ist Bier?','',1,'2022-04-11 12:00:00.000','2022-04-11 13:00:00.000','','','',0,NULL,'c33dfc976efb410299e604b255db0127','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','',0,1,NULL,NULL,0,NULL,NULL,NULL,'',NULL,NULL,NULL,NULL,0),
(2,'2022-04-18 13:46:25.841','2022-04-18 13:46:46.547',NULL,'VL 2: Wie mache ich Bier?','',1,'2022-04-18 12:00:00.000','2022-04-18 13:00:00.000','','','',0,NULL,'5815366e4010482687912588349bc5c0','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','',0,1,NULL,NULL,0,NULL,NULL,NULL,'',NULL,NULL,NULL,NULL,0),
(4,'2022-04-18 13:46:46.547','2022-04-18 13:46:46.547',NULL,'VL 3: Rückblick','',1,'2026-02-19 12:00:00.000','2026-02-19 13:00:00.000','','','',0,NULL,'d8ce0b882dbc4d999b42c143ce07db5a','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','',0,0,NULL,NULL,0,NULL,NULL,NULL,'',NULL,NULL,NULL,NULL,0),
(7,'2022-04-18 13:46:46.547','2022-04-18 13:46:46.547',NULL,'VL 1: Livestream','',2,'2022-02-19 12:00:00.000','2022-02-19 13:00:00.000','','','',0,NULL,'','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','',1,0,NULL,NULL,0,NULL,NULL,NULL,'',NULL,NULL,NULL,NULL,0),
(8,'2022-04-18 13:46:46.547','2022-04-18 13:46:46.547',NULL,'VL 1: Intro to Go','',3,'2022-02-19 12:00:00.000','2022-02-19 12:00:00.000','','','',0,NULL,'d8ce0b882dbc4d999b42c143ce07db5a','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','',0,1,NULL,NULL,0,NULL,NULL,NULL,'',NULL,NULL,NULL,NULL,0),
(9,'2022-04-18 13:46:46.547','2022-04-18 13:46:46.547',NULL,'Private stream','',4,'2022-02-19 12:00:00.000','2022-02-19 12:00:00.000','','','',0,NULL,'d8ce0b882dbc4d999b42c143ce07db5a','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','',0,1,NULL,NULL,0,NULL,NULL,NULL,'',NULL,NULL,NULL,NULL,0);
/*!40000 ALTER TABLE `streams` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `subtitles`
--

DROP TABLE IF EXISTS `subtitles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `subtitles` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `stream_id` bigint(20) unsigned NOT NULL,
  `content` longtext NOT NULL,
  `language` longtext NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_subtitles_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `subtitles`
--

LOCK TABLES `subtitles` WRITE;
/*!40000 ALTER TABLE `subtitles` DISABLE KEYS */;
/*!40000 ALTER TABLE `subtitles` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `tokens`
--

DROP TABLE IF EXISTS `tokens`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `tokens` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` bigint(20) unsigned DEFAULT NULL,
  `token` longtext NOT NULL,
  `expires` datetime(3) DEFAULT NULL,
  `scope` longtext NOT NULL,
  `last_use` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_tokens_deleted_at` (`deleted_at`),
  KEY `fk_tokens_user` (`user_id`),
  CONSTRAINT `fk_tokens_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `tokens`
--

LOCK TABLES `tokens` WRITE;
/*!40000 ALTER TABLE `tokens` DISABLE KEYS */;
/*!40000 ALTER TABLE `tokens` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `transcoding_failures`
--

DROP TABLE IF EXISTS `transcoding_failures`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `transcoding_failures` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `stream_id` bigint(20) unsigned NOT NULL,
  `version` longtext NOT NULL,
  `logs` longtext NOT NULL,
  `exit_code` bigint(20) DEFAULT NULL,
  `file_path` longtext NOT NULL,
  `hostname` longtext NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_transcoding_failures_deleted_at` (`deleted_at`),
  KEY `fk_transcoding_failures_stream` (`stream_id`),
  CONSTRAINT `fk_transcoding_failures_stream` FOREIGN KEY (`stream_id`) REFERENCES `streams` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `transcoding_failures`
--

LOCK TABLES `transcoding_failures` WRITE;
/*!40000 ALTER TABLE `transcoding_failures` DISABLE KEYS */;
/*!40000 ALTER TABLE `transcoding_failures` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `transcoding_progresses`
--

DROP TABLE IF EXISTS `transcoding_progresses`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `transcoding_progresses` (
  `stream_id` bigint(20) unsigned NOT NULL,
  `version` varchar(191) NOT NULL,
  `progress` bigint(20) NOT NULL DEFAULT 0,
  PRIMARY KEY (`stream_id`,`version`),
  CONSTRAINT `fk_streams_transcoding_progresses` FOREIGN KEY (`stream_id`) REFERENCES `streams` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `transcoding_progresses`
--

LOCK TABLES `transcoding_progresses` WRITE;
/*!40000 ALTER TABLE `transcoding_progresses` DISABLE KEYS */;
/*!40000 ALTER TABLE `transcoding_progresses` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `upload_keys`
--

DROP TABLE IF EXISTS `upload_keys`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `upload_keys` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `upload_key` longtext NOT NULL,
  `stream_id` bigint(20) unsigned DEFAULT NULL,
  `video_type` longtext NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_upload_keys_deleted_at` (`deleted_at`),
  KEY `fk_upload_keys_stream` (`stream_id`),
  CONSTRAINT `fk_upload_keys_stream` FOREIGN KEY (`stream_id`) REFERENCES `streams` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `upload_keys`
--

LOCK TABLES `upload_keys` WRITE;
/*!40000 ALTER TABLE `upload_keys` DISABLE KEYS */;
/*!40000 ALTER TABLE `upload_keys` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `user_settings`
--

DROP TABLE IF EXISTS `user_settings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `user_settings` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  `type` bigint(20) NOT NULL,
  `value` longtext NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_settings_deleted_at` (`deleted_at`),
  KEY `fk_users_settings` (`user_id`),
  CONSTRAINT `fk_users_settings` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=14 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `user_settings`
--

LOCK TABLES `user_settings` WRITE;
/*!40000 ALTER TABLE `user_settings` DISABLE KEYS */;
/*!40000 ALTER TABLE `user_settings` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `users`
--

DROP TABLE IF EXISTS `users`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `users` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `name` varchar(80) NOT NULL,
  `last_name` longtext DEFAULT NULL,
  `email` varchar(256) DEFAULT NULL,
  `matriculation_number` varchar(256) DEFAULT NULL,
  `lrz_id` longtext DEFAULT NULL,
  `role` bigint(20) unsigned DEFAULT 4,
  `password` varchar(191) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_users_email` (`email`),
  UNIQUE KEY `idx_users_matriculation_number` (`matriculation_number`),
  KEY `idx_users_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=9 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `users`
--

LOCK TABLES `users` WRITE;
/*!40000 ALTER TABLE `users` DISABLE KEYS */;
INSERT INTO `users` VALUES
(1,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'Anja Admin','admin','admin',NULL,NULL,1,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw'),
(2,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'Peter Prof','prof1','prof1',NULL,NULL,2,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw'),
(3,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'Pauline Prof','prof2','prof2',NULL,NULL,2,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw'),
(4,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'Stephanie Studi','studi1','studi1',NULL,NULL,4,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw'),
(5,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'Sven Studi','studi2','studi2',NULL,NULL,4,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw'),
(6,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'Sandra Studi','studi3','studi3',NULL,NULL,4,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw'),
(7,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'LoggedInUser','log','loggedin',NULL,NULL,4,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw'),
(8,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'EnrolledUser','enr','enrolled',NULL,NULL,4,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw');
/*!40000 ALTER TABLE `users` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `video_sections`
--

DROP TABLE IF EXISTS `video_sections`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `video_sections` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `description` varchar(191) NOT NULL,
  `start_hours` bigint(20) unsigned NOT NULL,
  `start_minutes` bigint(20) unsigned NOT NULL,
  `start_seconds` bigint(20) unsigned NOT NULL,
  `stream_id` bigint(20) unsigned NOT NULL,
  `file_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_video_sections_deleted_at` (`deleted_at`),
  KEY `fk_streams_video_sections` (`stream_id`),
  FULLTEXT KEY `idx_video_sections_description` (`description`),
  CONSTRAINT `fk_streams_video_sections` FOREIGN KEY (`stream_id`) REFERENCES `streams` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `video_sections`
--

LOCK TABLES `video_sections` WRITE;
/*!40000 ALTER TABLE `video_sections` DISABLE KEYS */;
/*!40000 ALTER TABLE `video_sections` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `video_seek_chunks`
--

DROP TABLE IF EXISTS `video_seek_chunks`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `video_seek_chunks` (
  `chunk_index` bigint(20) unsigned NOT NULL,
  `hits` bigint(20) unsigned NOT NULL,
  `stream_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`chunk_index`,`stream_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `video_seek_chunks`
--

LOCK TABLES `video_seek_chunks` WRITE;
/*!40000 ALTER TABLE `video_seek_chunks` DISABLE KEYS */;
INSERT INTO `video_seek_chunks` VALUES
(9223372036854775808,2,8);
/*!40000 ALTER TABLE `video_seek_chunks` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `workers`
--

DROP TABLE IF EXISTS `workers`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `workers` (
  `worker_id` varchar(191) NOT NULL,
  `host` longtext DEFAULT NULL,
  `status` longtext DEFAULT NULL,
  `workload` bigint(20) unsigned DEFAULT NULL,
  `last_seen` datetime(3) DEFAULT NULL,
  `cpu` longtext DEFAULT NULL,
  `memory` longtext DEFAULT NULL,
  `disk` longtext DEFAULT NULL,
  `uptime` longtext DEFAULT NULL,
  `version` longtext DEFAULT NULL,
  PRIMARY KEY (`worker_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `workers`
--

LOCK TABLES `workers` WRITE;
/*!40000 ALTER TABLE `workers` DISABLE KEYS */;
/*!40000 ALTER TABLE `workers` ENABLE KEYS */;
UNLOCK TABLES;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2024-01-03 21:50:27
