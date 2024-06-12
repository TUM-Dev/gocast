CREATE DATABASE IF NOT EXISTS `tumlive`;
USE `tumlive`;

-- MariaDB dump 10.19-11.3.2-MariaDB, for Linux (x86_64)
--
-- Host: localhost    Database: tumlive
-- ------------------------------------------------------
-- Server version	11.3.2-MariaDB

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
) ENGINE=InnoDB AUTO_INCREMENT=17 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `audits`
--

LOCK TABLES `audits` WRITE;
/*!40000 ALTER TABLE `audits` DISABLE KEYS */;
INSERT INTO `audits` VALUES
(1,'2024-06-02 22:24:42.407','2024-06-02 22:24:42.407',NULL,1,'Einführung Brauereiwesen:\'brauereiwesen\'',5),
(2,'2024-06-03 15:06:32.056','2024-06-03 15:06:32.056',NULL,1,'Praktikum: Golang:\'godev\' add: Anja Admin (1)',5),
(3,'2024-06-03 15:07:32.646','2024-06-03 15:07:32.646',NULL,1,'Praktikum: Golang:\'godev\' add: Max Maintainer (2)',5),
(4,'2024-06-03 15:07:37.214','2024-06-03 15:07:37.214',NULL,1,'Praktikum: Golang:\'godev\' remove: Max Maintainer (2)',5),
(5,'2024-06-03 15:07:53.837','2024-06-03 15:07:53.837',NULL,1,'8: (Visibility: true)',8),
(6,'2024-06-03 15:08:40.288','2024-06-03 15:08:40.288',NULL,1,'8: (Visibility: false)',8),
(7,'2024-06-03 15:47:42.113','2024-06-03 15:47:42.113',NULL,1,'eidi:\'Einführung in die Informatik\' (2024, W)',4),
(8,'2024-06-03 15:50:28.559','2024-06-03 15:50:28.559',NULL,1,'eidi:\'Einführung in die Informatik\' (2024, W)',4),
(9,'2024-06-03 20:16:46.622','2024-06-03 20:16:46.622',NULL,2,'hw:\'hello world\' (2024, W)',4),
(10,'2024-06-03 20:19:55.066','2024-06-03 20:19:55.066',NULL,2,'test:\'test\' (2024, W)',4),
(11,'2024-06-03 20:20:28.577','2024-06-03 20:20:28.577',NULL,2,'test2:\'test\' (2024, W)',4),
(12,'2024-06-03 20:22:05.504','2024-06-03 20:22:05.504',NULL,2,'awd:\'test\' (2024, W)',4),
(13,'2024-06-03 20:22:50.212','2024-06-03 20:22:50.212',NULL,2,'awdaa:\'test\' (2024, W)',4),
(14,'2024-06-03 21:37:37.498','2024-06-03 21:37:37.498',NULL,1,'\'Einführung Brauereiwesen\' (2022, S)[1]',6),
(15,'2024-06-03 22:10:41.425','2024-06-03 22:10:41.425',NULL,1,'\'\' (0, )[0]',6),
(16,'2024-06-03 22:15:17.715','2024-06-03 22:15:17.715',NULL,1,'\'test\' (2024, W)[8]',6);
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
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
(3,1),
(1,3),
(2,3),
(12,3);
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
(3,6);
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
  `school_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_courses_deleted_at` (`deleted_at`),
  KEY `fk_schools_courses` (`school_id`),
  CONSTRAINT `fk_schools_courses` FOREIGN KEY (`school_id`) REFERENCES `schools` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=13 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `courses`
--

LOCK TABLES `courses` WRITE;
/*!40000 ALTER TABLE `courses` DISABLE KEYS */;
INSERT INTO `courses` VALUES
(1,'2022-04-18 13:40:05.843','2024-06-03 21:37:37.511',NULL,1,'Einführung Brauereiwesen','brauereiwesen',2022,'S','',0,1,0,0,0,0,'public','',0,'null','[{\"lecture_hall_id\":1,\"source_mode\":1}]',0,0,2),
(2,'2022-04-18 13:40:54.686','2024-06-03 22:51:43.338',NULL,1,'Spieleentwicklung für Dummies','games101',2022,'S','',1,1,0,0,0,0,'loggedin','',0,'','',0,0,2),
(3,'2022-04-18 13:41:55.741','2024-06-03 21:59:53.802',NULL,1,'Praktikum: Golang','godev',2021,'W','',1,1,0,0,0,0,'public','',0,'','',1,0,2),
(5,'2024-06-03 15:50:28.569','2024-06-03 15:50:33.642',NULL,1,'Einführung in die Informatik','eidi',2024,'W','',1,0,0,1,0,0,'loggedin','',0,'','',0,0,2),
(6,'2024-06-03 20:16:46.623','2024-06-03 20:16:46.682',NULL,2,'hello world','hw',2024,'W','',1,0,0,1,0,0,'loggedin','',0,'','',0,0,2),
(7,'2024-06-03 20:19:55.076','2024-06-03 20:19:55.116',NULL,2,'test','test',2024,'W','',1,0,0,1,0,0,'loggedin','',0,'','',0,0,2),
(8,'2024-06-03 20:20:28.587','2024-06-03 22:15:17.723','2024-06-03 22:15:17.725',2,'test','test2',2024,'W','',0,0,0,1,0,0,'loggedin','',0,'','',0,0,2),
(9,'2024-06-03 20:22:05.513','2024-06-03 22:27:11.254',NULL,2,'test','awd',2024,'W','',1,0,0,1,0,0,'loggedin','',0,'','',0,0,2),
(10,'2024-06-03 20:22:50.222','2024-06-03 20:22:50.259',NULL,2,'test','awdaa',2024,'W','',1,0,0,1,0,0,'loggedin','',0,'','',0,0,2),
(12,'2024-06-03 21:45:50.145','2024-06-03 22:51:07.004',NULL,1,'Spieleentwicklung für Dummies','games101',2022,'W','',1,1,0,1,0,0,'loggedin','',0,'','',0,0,2);
/*!40000 ALTER TABLE `courses` ENABLE KEYS */;
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
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
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
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `lecture_halls`
--

LOCK TABLES `lecture_halls` WRITE;
/*!40000 ALTER TABLE `lecture_halls` DISABLE KEYS */;
INSERT INTO `lecture_halls` VALUES
(1,NULL,NULL,NULL,'HS001','Hörsaal 001',NULL,NULL,NULL,NULL,NULL,NULL,NULL,1,NULL),
(2,'2024-06-02 22:23:32.731','2024-06-02 22:23:32.731',NULL,'FMI_HS1','','0.0.0.0','0.0.0.0','0.0.0.0','0.0.0.0',0,'0.0.0.0',0,1,'');
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
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `register_links`
--

LOCK TABLES `register_links` WRITE;
/*!40000 ALTER TABLE `register_links` DISABLE KEYS */;
INSERT INTO `register_links` VALUES
(1,'2024-06-02 18:03:58.574','2024-06-02 18:03:58.574',NULL,7,'c795e691-75db-4eed-ab17-06cab2d1abd9'),
(2,'2024-06-03 01:00:06.774','2024-06-03 01:00:06.774',NULL,8,'7681c9b1-9a75-4bfe-8456-e36151e8b5b1');
/*!40000 ALTER TABLE `register_links` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `runners`
--

DROP TABLE IF EXISTS `runners`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `runners` (
  `hostname` varchar(191) NOT NULL,
  `port` bigint(20) DEFAULT NULL,
  `last_seen` datetime(3) DEFAULT NULL,
  `status` longtext DEFAULT NULL,
  `workload` bigint(20) unsigned DEFAULT NULL,
  `cpu` longtext DEFAULT NULL,
  `memory` longtext DEFAULT NULL,
  `disk` longtext DEFAULT NULL,
  `uptime` longtext DEFAULT NULL,
  `version` longtext DEFAULT NULL,
  `school_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`hostname`),
  KEY `fk_schools_runner` (`school_id`),
  CONSTRAINT `fk_schools_runner` FOREIGN KEY (`school_id`) REFERENCES `schools` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `runners`
--

LOCK TABLES `runners` WRITE;
/*!40000 ALTER TABLE `runners` DISABLE KEYS */;
/*!40000 ALTER TABLE `runners` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `school_admins`
--

DROP TABLE IF EXISTS `school_admins`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `school_admins` (
  `school_id` bigint(20) unsigned NOT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`school_id`,`user_id`),
  KEY `fk_school_admins_user` (`user_id`),
  CONSTRAINT `fk_school_admins_school` FOREIGN KEY (`school_id`) REFERENCES `schools` (`id`),
  CONSTRAINT `fk_school_admins_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `school_admins`
--

LOCK TABLES `school_admins` WRITE;
/*!40000 ALTER TABLE `school_admins` DISABLE KEYS */;
INSERT INTO `school_admins` VALUES
(1,1),
(2,1),
(3,1),
(3,8);
(4,1),
(4,2),
/*!40000 ALTER TABLE `school_admins` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `schools`
--

DROP TABLE IF EXISTS `schools`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `schools` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `name` text NOT NULL,
  `university` text NOT NULL DEFAULT 'unknown',
  `shared_resources_allowed` tinyint(1) NOT NULL DEFAULT 0,
  `privileges` text NOT NULL DEFAULT '',
  `tum_online_id` text NOT NULL DEFAULT '',
  PRIMARY KEY (`id`),
  KEY `idx_schools_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=14 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `schools`
--

LOCK TABLES `schools` WRITE;
/*!40000 ALTER TABLE `schools` DISABLE KEYS */;
INSERT INTO `schools` VALUES
(1,'2024-06-02 21:26:35.075','2024-06-03 01:19:08.273',NULL,'master','service',0,'',''),
(2,'2024-06-12 02:06:08.239','2024-06-12 02:06:08.250',NULL,'CIT','TUM',0,'',''),
(3,'2024-06-12 02:06:17.797','2024-06-12 02:13:11.199',NULL,'IFI','LMU',0,'',''),
(4,'2024-06-12 02:06:24.127','2024-06-12 02:12:09.510',NULL,'MGMT','TUM',0,'','');
/*!40000 ALTER TABLE `schools` ENABLE KEYS */;
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
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `silences`
--

LOCK TABLES `silences` WRITE;
/*!40000 ALTER TABLE `silences` DISABLE KEYS */;
INSERT INTO `silences` VALUES
(1,'2024-01-14 21:16:37.000','2024-01-14 21:16:43.000',NULL,0,100,1),
(2,'2024-01-14 21:17:00.000','2024-01-14 21:17:02.000',NULL,0,200,2);
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
) ENGINE=InnoDB AUTO_INCREMENT=35 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
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
(9,'2022-04-18 13:57:00.002','2022-04-18 13:57:00.002',NULL,'2022-04-18 13:57:00.000',7,0,1),
(10,'2024-06-03 00:01:00.005','2024-06-03 00:01:00.005',NULL,'2024-06-03 00:01:00.003',7,1,1),
(11,'2024-06-03 01:21:00.008','2024-06-03 01:21:00.008',NULL,'2024-06-03 01:21:00.004',7,1,1),
(12,'2024-06-03 01:22:00.007','2024-06-03 01:22:00.007',NULL,'2024-06-03 01:22:00.003',7,1,1),
(13,'2024-06-03 01:23:00.006','2024-06-03 01:23:00.006',NULL,'2024-06-03 01:23:00.003',7,1,1),
(14,'2024-06-03 01:24:00.005','2024-06-03 01:24:00.005',NULL,'2024-06-03 01:24:00.002',7,1,1),
(15,'2024-06-03 01:25:00.007','2024-06-03 01:25:00.007',NULL,'2024-06-03 01:25:00.003',7,1,1),
(16,'2024-06-03 01:26:00.005','2024-06-03 01:26:00.005',NULL,'2024-06-03 01:26:00.004',7,1,1),
(17,'2024-06-03 01:27:00.006','2024-06-03 01:27:00.006',NULL,'2024-06-03 01:27:00.004',7,1,1),
(18,'2024-06-03 01:28:00.005','2024-06-03 01:28:00.005',NULL,'2024-06-03 01:28:00.003',7,1,1),
(19,'2024-06-03 01:29:00.005','2024-06-03 01:29:00.005',NULL,'2024-06-03 01:29:00.003',7,1,1),
(20,'2024-06-03 01:30:00.005','2024-06-03 01:30:00.005',NULL,'2024-06-03 01:30:00.004',7,1,1),
(21,'2024-06-03 01:31:00.006','2024-06-03 01:31:00.006',NULL,'2024-06-03 01:31:00.003',7,1,1),
(22,'2024-06-03 01:32:00.005','2024-06-03 01:32:00.005',NULL,'2024-06-03 01:32:00.003',7,1,1),
(23,'2024-06-03 01:33:00.005','2024-06-03 01:33:00.005',NULL,'2024-06-03 01:33:00.003',7,1,1),
(24,'2024-06-03 01:34:00.006','2024-06-03 01:34:00.006',NULL,'2024-06-03 01:34:00.004',7,1,1),
(25,'2024-06-03 01:35:00.006','2024-06-03 01:35:00.006',NULL,'2024-06-03 01:35:00.004',7,1,1),
(26,'2024-06-03 01:36:00.005','2024-06-03 01:36:00.005',NULL,'2024-06-03 01:36:00.003',7,1,1),
(27,'2024-06-03 01:37:00.005','2024-06-03 01:37:00.005',NULL,'2024-06-03 01:37:00.002',7,1,1),
(28,'2024-06-03 01:38:00.006','2024-06-03 01:38:00.006',NULL,'2024-06-03 01:38:00.004',7,1,1),
(29,'2024-06-03 01:39:00.005','2024-06-03 01:39:00.005',NULL,'2024-06-03 01:39:00.003',7,1,1),
(30,'2024-06-03 01:40:00.006','2024-06-03 01:40:00.006',NULL,'2024-06-03 01:40:00.004',7,1,1),
(31,'2024-06-03 01:41:00.005','2024-06-03 01:41:00.005',NULL,'2024-06-03 01:41:00.004',7,1,1),
(32,'2024-06-03 01:51:00.005','2024-06-03 01:51:00.005',NULL,'2024-06-03 01:51:00.003',7,1,1),
(33,'2024-06-03 01:52:00.004','2024-06-03 01:52:00.004',NULL,'2024-06-03 01:52:00.002',7,1,1),
(34,'2024-06-03 15:25:19.612','2024-06-03 15:55:51.426',NULL,'2024-06-03 15:00:00.000',2,2,0);
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
  `freed_at` datetime(3) NOT NULL DEFAULT '2024-06-12 00:00:00.000',
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
(0,0,1,1),
(0.03855875011106437,0,2,2),
(0.3546924531559881,0,8,2);
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
  `requested` tinyint(1) DEFAULT 0,
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
(1,'2022-04-18 13:45:58.657','2024-06-02 22:24:42.419','2024-06-03 21:37:37.510','VL 1: Was ist Bier?','',1,'2022-04-11 12:00:00.000','2022-04-11 12:09:56.000','','','',0,NULL,'c33dfc976efb410299e604b255db0127','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','',0,1,NULL,NULL,0,NULL,NULL,1,'',NULL,NULL,NULL,NULL,0,0),
(2,'2022-04-18 13:46:25.841','2024-06-02 22:24:42.419','2024-06-03 21:37:37.509','VL 2: Wie mache ich Bier?','',1,'2022-04-18 12:00:00.000','2022-04-18 12:09:56.000','','','',0,NULL,'5815366e4010482687912588349bc5c0','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','',0,1,NULL,NULL,0,NULL,NULL,1,'',NULL,NULL,NULL,NULL,0,0),
(4,'2022-04-18 13:46:46.547','2024-06-02 22:24:42.419','2024-06-03 21:37:37.507','VL 3: Rückblick','',1,'2026-02-19 12:00:00.000','2026-02-19 13:00:00.000','','','',0,NULL,'d8ce0b882dbc4d999b42c143ce07db5a','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','',0,0,NULL,NULL,0,NULL,NULL,1,'',NULL,NULL,NULL,NULL,0,0),
(7,'2022-04-18 13:46:46.547','2024-06-03 22:51:43.339',NULL,'VL 1: Livestream','',2,'2022-02-19 12:00:00.000','2022-02-19 13:00:00.000','','','',0,NULL,'','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','',1,0,NULL,NULL,0,NULL,NULL,1,'',NULL,NULL,NULL,NULL,0,0),
(8,'2022-04-18 13:46:46.547','2024-06-03 15:08:40.296',NULL,'VL 1: Intro to Go','',3,'2022-02-19 12:00:00.000','2022-02-19 12:00:00.000','','','',0,NULL,'d8ce0b882dbc4d999b42c143ce07db5a','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','',0,1,NULL,NULL,0,NULL,NULL,1,'',NULL,1,NULL,NULL,0,0),
(9,'2024-06-03 21:45:50.164','2024-06-03 22:51:07.005',NULL,'VL 1: Livestream','',12,'2022-02-19 12:00:00.000','2022-02-19 13:00:00.000','','','',0,NULL,'','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8','https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/bb.mp4/playlist.m3u8',NULL,1,0,NULL,NULL,0,NULL,NULL,1,'',NULL,NULL,NULL,NULL,0,0);
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
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
  `email` varchar(256) DEFAULT NULL,
  `matriculation_number` varchar(256) DEFAULT NULL,
  `lrz_id` longtext DEFAULT NULL,
  `role` bigint(20) unsigned DEFAULT 5,
  `password` varchar(191) DEFAULT NULL,
  `last_name` longtext DEFAULT NULL,
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
(1,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'Anja Admin','admin',NULL,NULL,1,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw',NULL),
(2,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'Max Maintainer','mgmt_maintainer',NULL,NULL,2,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw',NULL),
(3,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'Pauline Prof','prof1',NULL,NULL,3,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw',NULL),
(4,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'Stephanie Studi','studi1',NULL,NULL,5,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw',NULL),
(5,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'Sven Studi','studi2',NULL,NULL,5,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw',NULL),
(6,'2022-04-18 13:36:21.000','2022-04-18 13:36:22.000',NULL,'Sandra Studi','studi3',NULL,NULL,5,'$argon2id$v=19$m=65536,t=3,p=2$r/ST3fAucfj+DfrH9Rc8Eg$xqL7eHttIkhpXUq8VxqyMc6/H9HnorNYFNqWyXdj2iw',NULL),
(7,'2024-06-02 18:03:58.564','2024-06-02 18:03:58.564',NULL,'Max Mustermann','admin2',NULL,'',1,NULL,NULL),
(8,'2024-06-03 01:00:06.763','2024-06-03 01:00:06.763',NULL,'Ludwig Maintainer','lmu_maintainer',NULL,'',2,NULL,NULL);
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
(9223372036854775808,2,2),
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
  `school_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`worker_id`),
  KEY `fk_schools_workers` (`school_id`),
  CONSTRAINT `fk_schools_workers` FOREIGN KEY (`school_id`) REFERENCES `schools` (`id`)
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

-- Dump completed on 2024-06-12  2:14:36
