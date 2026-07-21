-- 仅允许本地业务账号在 GORM Gen 随机临时数据库内执行迁移和反向生成。
GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, INDEX ON `kratos\_admin\_gen\_%`.* TO 'kratos'@'%';
