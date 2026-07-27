package com.aicontrolplane.databridge.config;

import dev.langchain4j.data.segment.TextSegment;
import dev.langchain4j.model.embedding.EmbeddingModel;
import dev.langchain4j.model.embedding.onnx.allminilml6v2.AllMiniLmL6V2EmbeddingModel;
import dev.langchain4j.store.embedding.EmbeddingStore;
import dev.langchain4j.store.embedding.pgvector.PgVectorEmbeddingStore;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Manual bean configuration for LangChain4j pgvector and embedding model.
 * (No auto-starter exists for pgvector at 1.0.0-beta1.)
 */
@Configuration
public class LangChainConfig {

    @Value("${spring.datasource.url}")
    private String dbUrl;

    @Value("${spring.datasource.username}")
    private String dbUser;

    @Value("${spring.datasource.password}")
    private String dbPassword;

    /**
     * Local in-process embedding model — all-MiniLM-L6-v2.
     * Zero external API cost, data never leaves the container.
     * Output dimension: 384.
     */
    @Bean
    public EmbeddingModel embeddingModel() {
        return new AllMiniLmL6V2EmbeddingModel();
    }

    /**
     * pgvector-backed embedding store.
     * Connects to the same Postgres instance, uses the "embeddings" table.
     */
    @Bean
    public EmbeddingStore<TextSegment> embeddingStore() {
        // Parse host, port, and database from JDBC URL
        // Format: jdbc:postgresql://host:port/database
        String cleanUrl = dbUrl.replace("jdbc:postgresql://", "");
        String[] hostPortDb = cleanUrl.split("[:/]");
        String host = hostPortDb[0];
        int port = Integer.parseInt(hostPortDb[1]);
        String database = hostPortDb[2];

        return PgVectorEmbeddingStore.builder()
                .host(host)
                .port(port)
                .database(database)
                .user(dbUser)
                .password(dbPassword)
                .table("embeddings")
                .dimension(384)
                .build();
    }
}
