package com.aicontrolplane.databridge.config;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;

import javax.sql.DataSource;

/**
 * Configures a read-only DataSource and JdbcTemplate for Text-to-SQL.
 * Defense in depth: even if the AI generates DROP/INSERT/UPDATE,
 * the ai_reader user has only SELECT privileges.
 */
@Configuration
public class ReadOnlyDataSourceConfig {

    @Value("${app.datasource.readonly.url}")
    private String url;

    @Value("${app.datasource.readonly.username}")
    private String username;

    @Value("${app.datasource.readonly.password}")
    private String password;

    @Bean(name = "readOnlyDataSource")
    public DataSource readOnlyDataSource() {
        DriverManagerDataSource ds = new DriverManagerDataSource();
        ds.setDriverClassName("org.postgresql.Driver");
        ds.setUrl(url);
        ds.setUsername(username);
        ds.setPassword(password);
        return ds;
    }

    @Bean(name = "readOnlyJdbcTemplate")
    public JdbcTemplate readOnlyJdbcTemplate() {
        return new JdbcTemplate(readOnlyDataSource());
    }
}
