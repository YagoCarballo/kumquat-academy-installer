# ************************************************************
# Sequel Pro SQL dump
# Version 4499
#
# http://www.sequelpro.com/
# https://github.com/sequelpro/sequelpro
#
# Host: localhost (MySQL 5.6.26)
# Database: Individual-Project
# Generation Time: 2015-11-07 18:50:21 +0000
# ************************************************************

# Dump of table sessions
# ------------------------------------------------------------

LOCK TABLES `sessions` WRITE;
/*!40000 ALTER TABLE `sessions` DISABLE KEYS */;

INSERT INTO `sessions` (`token`, `user_id`, `ip`, `expires_in`, `created_on`)
VALUES
	('a077c80d-77e2-4328-80c4-f2b4ccf995c4',6,'127.0.0.1','2015-10-31 18:15:51','2015-10-24 17:15:51'),
	('d1c3d3c6-7aab-4690-866f-7a17bb76fc1a',6,'[','2015-11-07 11:37:12','2015-10-31 11:37:12');

/*!40000 ALTER TABLE `sessions` ENABLE KEYS */;
UNLOCK TABLES;


# Dump of table users
# ------------------------------------------------------------

LOCK TABLES `users` WRITE;
/*!40000 ALTER TABLE `users` DISABLE KEYS */;

INSERT INTO `users` (`id`, `username`, `password`, `email`, `first_name`, `last_name`, `date_of_birth`, `matric_number`, `matric_date`, `active`)
VALUES
	(6,'admin','$2a$10$gjrXY99EdUI9ndhpVbzfbe4Xb2MkzzzEtkJ8EcZc.V0ijRjx36oXC','john@doe.com','John','Doe','1990-05-18','123456789','2015-10-24 18:00:46',0);

/*!40000 ALTER TABLE `users` ENABLE KEYS */;
UNLOCK TABLES;
