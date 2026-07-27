package com.aicontrolplane.databridge.service;

import dev.langchain4j.model.chat.ChatLanguageModel;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Service;

import java.util.List;
import java.util.Map;

/**
 * Text-to-SQL Service — translates natural language questions into SQL
 * using Gemini, then executes against a read-only datasource.
 *
 * <p>Double safety:
 * <ul>
 *   <li>Code-level: rejects any query that isn't a SELECT</li>
 *   <li>DB-level: ai_reader user has SELECT-only permissions</li>
 * </ul>
 */
@Service
public class TextToSqlService {

    private static final Logger log = LoggerFactory.getLogger(TextToSqlService.class);

    private final ChatLanguageModel chatModel;
    private final JdbcTemplate readOnlyJdbc;

    /**
     * Schema-with-context system prompt — tells the LLM about column types,
     * units, and formats for higher SQL accuracy.
     */
    private static final String SYSTEM_PROMPT = """
            You are an expert SQL assistant. Convert the user's natural language question
            into a valid PostgreSQL SELECT query. Return ONLY the raw SQL, no explanation.

            Available table:

            Table: company_sales
            Columns:
              - id: SERIAL (auto-increment primary key)
              - product: VARCHAR(100) — product name (e.g., 'Enterprise AI Suite', 'Data Analytics Pro', 'Cloud Security Plus', 'ML Pipeline Toolkit', 'Automation Hub')
              - region: VARCHAR(50) — always Title Case (North, South, East, West)
              - amount: DECIMAL(12,2) — sale amount in USD
              - sale_date: DATE — format YYYY-MM-DD
              - salesperson: VARCHAR(100) — full name of the salesperson

            Rules:
            1. ONLY generate SELECT queries. Never generate INSERT, UPDATE, DELETE, DROP, ALTER, or any DDL/DML.
            2. Use proper PostgreSQL syntax.
            3. For date filters, use ISO format (YYYY-MM-DD).
            4. If the question is ambiguous, make reasonable assumptions.
            5. Return ONLY the SQL query, nothing else.
            """;

    public TextToSqlService(ChatLanguageModel chatModel,
                            @Qualifier("readOnlyJdbcTemplate") JdbcTemplate readOnlyJdbc) {
        this.chatModel = chatModel;
        this.readOnlyJdbc = readOnlyJdbc;
    }

    /**
     * Result record holding the generated SQL, query results, and row count.
     */
    public record SqlResult(String sql, List<Map<String, Object>> results, int rowCount, String error) {
        public static SqlResult success(String sql, List<Map<String, Object>> results) {
            return new SqlResult(sql, results, results.size(), null);
        }

        public static SqlResult error(String error) {
            return new SqlResult(null, List.of(), 0, error);
        }
    }

    /**
     * Takes a natural language question, generates SQL via Gemini,
     * validates it, and executes against the read-only database.
     */
    public SqlResult ask(String question) {
        if (question == null || question.isBlank()) {
            return SqlResult.error("Question cannot be empty.");
        }

        try {
            // Step 1: Generate SQL from natural language
            String fullPrompt = SYSTEM_PROMPT + "\n\nUser question: " + question;
            String generatedSql = chatModel.chat(fullPrompt).trim();

            // Clean up: remove markdown code fences if present
            generatedSql = generatedSql
                    .replaceAll("(?s)```sql\\s*", "")
                    .replaceAll("(?s)```\\s*", "")
                    .trim();

            log.info("[TEXT-TO-SQL] Question: '{}' → SQL: '{}'", question, generatedSql);

            // Step 2: Safety check — code-level SELECT-only enforcement
            String upperSql = generatedSql.toUpperCase().trim();
            if (!upperSql.startsWith("SELECT")) {
                log.warn("[TEXT-TO-SQL] BLOCKED non-SELECT query: {}", generatedSql);
                return SqlResult.error("Only SELECT queries are allowed. The generated query was blocked for safety.");
            }

            // Reject dangerous keywords even within SELECT (e.g., subqueries with DROP)
            String[] blockedKeywords = {"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "TRUNCATE", "CREATE", "GRANT", "REVOKE"};
            for (String keyword : blockedKeywords) {
                if (upperSql.contains(keyword)) {
                    log.warn("[TEXT-TO-SQL] BLOCKED query containing '{}': {}", keyword, generatedSql);
                    return SqlResult.error("Query blocked: contains forbidden keyword '" + keyword + "'.");
                }
            }

            // Step 3: Execute against read-only datasource
            List<Map<String, Object>> results = readOnlyJdbc.queryForList(generatedSql);

            log.info("[TEXT-TO-SQL] Returned {} rows", results.size());
            return SqlResult.success(generatedSql, results);

        } catch (Exception e) {
            log.error("[TEXT-TO-SQL] Error: {}", e.getMessage(), e);
            return SqlResult.error("Failed to process query: " + e.getMessage());
        }
    }
}
