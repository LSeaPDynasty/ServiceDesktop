package services

// DefaultRegistry 返回预置的 8 个服务模板
func DefaultRegistry() []ServiceTemplate {
	return []ServiceTemplate{
		tomcatTemplate(),
		redisTemplate(),
		kafkaTemplate(),
		nacosTemplate(),
		nginxTemplate(),
		mysqlTemplate(),
		postgresTemplate(),
		mongodbTemplate(),
	}
}

// ---------- 中间件 ---------- //

func tomcatTemplate() ServiceTemplate {
	return ServiceTemplate{
		Service: Service{
			ID:          "tomcat",
			Name:        "Tomcat",
			DisplayName: "Apache Tomcat",
			Category:    CategoryMiddleware,
			StartCmd:    `{install_path}\bin\startup.bat`,
			StopCmd:     `{install_path}\bin\shutdown.bat`,
			Port:        8080,
			LogFile:     `{install_path}\logs\`,
			Args:        []string{},
			EnvVars: map[string]string{
				"CATALINA_HOME": "{install_path}",
				"JAVA_HOME":     "",
			},
			IsTemplate: true,
		},
		Description: "Java Servlet 容器，支持部署多个 WAR/JAR 应用",
		HomeVar:     "CATALINA_HOME",
		DefaultPort: 8080,
		DetectPaths: []string{
			`C:\Program Files\Apache Software Foundation\Tomcat *`,
			`C:\tools\tomcat*`,
			`D:\tools\tomcat*`,
		},
	}
}

func kafkaTemplate() ServiceTemplate {
	return ServiceTemplate{
		Service: Service{
			ID:          "kafka",
			Name:        "Kafka",
			DisplayName: "Apache Kafka",
			Category:    CategoryMiddleware,
			StartCmd:    `{install_path}\bin\windows\kafka-server-start.bat`,
			StopCmd:     `{install_path}\bin\windows\kafka-server-stop.bat`,
			Port:        9092,
			LogFile:     `{install_path}\logs\`,
			Args:        []string{`{install_path}\config\server.properties`},
			EnvVars: map[string]string{
				"KAFKA_HEAP_OPTS": "-Xmx1G -Xms1G",
			},
			IsTemplate: true,
		},
		Description: "分布式消息队列，配合 ZooKeeper 使用",
		HomeVar:     "KAFKA_HOME",
		DefaultPort: 9092,
		DetectPaths: []string{
			`C:\tools\kafka*`,
			`D:\tools\kafka*`,
		},
	}
}

func nacosTemplate() ServiceTemplate {
	return ServiceTemplate{
		Service: Service{
			ID:          "nacos",
			Name:        "Nacos",
			DisplayName: "Nacos",
			Category:    CategoryMiddleware,
			StartCmd:    `{install_path}\bin\startup.cmd`,
			StopCmd:     `{install_path}\bin\shutdown.cmd`,
			Port:        8848,
			LogFile:     `{install_path}\logs\`,
			Args:        []string{"-m", "standalone"},
			EnvVars: map[string]string{
				"MODE":                  "standalone",
				"NACOS_HOME":            "{install_path}",
				"JAVA_HOME":             "",
			},
			IsTemplate: true,
		},
		Description: "阿里云开源的动态服务发现、配置和服务管理平台",
		HomeVar:     "NACOS_HOME",
		DefaultPort: 8848,
		DetectPaths: []string{
			`C:\tools\nacos*`,
			`D:\tools\nacos*`,
		},
	}
}

func nginxTemplate() ServiceTemplate {
	return ServiceTemplate{
		Service: Service{
			ID:          "nginx",
			Name:        "Nginx",
			DisplayName: "Nginx",
			Category:    CategoryMiddleware,
			StartCmd:    `{install_path}\nginx.exe`,
			StopCmd:     `{install_path}\nginx.exe -s stop`,
			Port:        80,
			LogFile:     `{install_path}\logs\`,
			Args:        []string{},
			EnvVars:     map[string]string{},
			IsTemplate:  true,
		},
		Description: "高性能 HTTP 服务器和反向代理",
		HomeVar:     "NGINX_HOME",
		DefaultPort: 80,
		DetectPaths: []string{
			`C:\tools\nginx*`,
			`C:\nginx*`,
			`D:\tools\nginx*`,
		},
	}
}

// ---------- 数据库 ---------- //

func redisTemplate() ServiceTemplate {
	return ServiceTemplate{
		Service: Service{
			ID:          "redis",
			Name:        "Redis",
			DisplayName: "Redis",
			Category:    CategoryDatabase,
			StartCmd:    `{install_path}\redis-server.exe`,
			StopCmd:     `{install_path}\redis-cli.exe shutdown`,
			Port:        6379,
			LogFile:     `{install_path}\logs\`,
			Args:        []string{`{install_path}\redis.conf`},
			EnvVars:     map[string]string{},
			IsTemplate:  true,
		},
		Description: "内存键值数据库，常用于缓存",
		HomeVar:     "REDIS_HOME",
		DefaultPort: 6379,
		DetectPaths: []string{
			`C:\tools\redis*`,
			`D:\tools\redis*`,
		},
	}
}

func mysqlTemplate() ServiceTemplate {
	return ServiceTemplate{
		Service: Service{
			ID:          "mysql",
			Name:        "MySQL",
			DisplayName: "MySQL",
			Category:    CategoryDatabase,
			StartCmd:    `{install_path}\bin\mysqld.exe`,
			StopCmd:     `{install_path}\bin\mysqladmin.exe -u root shutdown`,
			Port:        3306,
			LogFile:     `{install_path}\data\`,
			Args:        []string{},
			EnvVars: map[string]string{
				"MYSQL_HOME": "{install_path}",
			},
			IsTemplate: true,
		},
		Description: "最流行的开源关系型数据库",
		HomeVar:     "MYSQL_HOME",
		DefaultPort: 3306,
		DetectPaths: []string{
			`C:\Program Files\MySQL\MySQL Server *`,
			`C:\tools\mysql*`,
			`D:\tools\mysql*`,
		},
	}
}

func postgresTemplate() ServiceTemplate {
	return ServiceTemplate{
		Service: Service{
			ID:          "postgresql",
			Name:        "PostgreSQL",
			DisplayName: "PostgreSQL",
			Category:    CategoryDatabase,
			StartCmd:    `{install_path}\bin\pg_ctl.exe start -D {install_path}\data`,
			StopCmd:     `{install_path}\bin\pg_ctl.exe stop -D {install_path}\data`,
			Port:        5432,
			LogFile:     `{install_path}\data\pg_log\`,
			Args:        []string{},
			EnvVars: map[string]string{
				"PGDATA": "{install_path}\\data",
			},
			IsTemplate: true,
		},
		Description: "功能强大的开源关系型数据库",
		HomeVar:     "PG_HOME",
		DefaultPort: 5432,
		DetectPaths: []string{
			`C:\Program Files\PostgreSQL\*`,
			`C:\tools\pgsql*`,
			`D:\tools\pgsql*`,
		},
	}
}

func mongodbTemplate() ServiceTemplate {
	return ServiceTemplate{
		Service: Service{
			ID:          "mongodb",
			Name:        "MongoDB",
			DisplayName: "MongoDB",
			Category:    CategoryDatabase,
			StartCmd:    `{install_path}\bin\mongod.exe`,
			StopCmd:     `{install_path}\bin\mongod.exe --shutdown`,
			Port:        27017,
			LogFile:     `{install_path}\data\log\`,
			Args: []string{
				"--dbpath", `{install_path}\data\db`,
				"--logpath", `{install_path}\data\log\mongod.log`,
			},
			EnvVars: map[string]string{},
			IsTemplate: true,
		},
		Description: "文档型 NoSQL 数据库，适合快速原型开发",
		HomeVar:     "MONGODB_HOME",
		DefaultPort: 27017,
		DetectPaths: []string{
			`C:\Program Files\MongoDB\Server\*`,
			`C:\tools\mongodb*`,
			`D:\tools\mongodb*`,
		},
	}
}
