package server

import (
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
)

type QueryColumn int

const (
	QueryColumnPayload QueryColumn = iota
	QueryColumnTimestamp
	QueryColumnLogMessageID
)

const (
	BigQueryTimestampFormat = "2006-01-02 15:04:05.999999 UTC"
)

type BigQueryTableQueryBuilder struct {
	client *bigquery.Client
	table  *bigquery.Table

	parameters map[string]bigquery.QueryParameter
}

func (qb *BigQueryTableQueryBuilder) WithClient(client *bigquery.Client) {
	qb.client = client
}

func (qb *BigQueryTableQueryBuilder) WithTable(table *bigquery.Table) {
	qb.table = table
}

func (qb *BigQueryTableQueryBuilder) WithLog(logID string) {
	qb.parameters["logID"] = bigquery.QueryParameter{
		Name:  "logID",
		Value: logID,
	}
}

func (qb *BigQueryTableQueryBuilder) WithEncryptionKey(encryptionKey string) {
	qb.parameters["encryptionKey"] = bigquery.QueryParameter{
		Name:  "encryptionKey",
		Value: encryptionKey,
	}
}

func (qb *BigQueryTableQueryBuilder) WithStartAt(time *time.Time) {
	qb.parameters["startAt"] = bigquery.QueryParameter{
		Name:  "startAt",
		Value: time.Format(BigQueryTimestampFormat),
	}
}

func (qb *BigQueryTableQueryBuilder) WithEndAt(time *time.Time) {
	qb.parameters["endAt"] = bigquery.QueryParameter{
		Name:  "endAt",
		Value: time.Format(BigQueryTimestampFormat),
	}
}

func (qb *BigQueryTableQueryBuilder) After(time *time.Time) {
	qb.parameters["after"] = bigquery.QueryParameter{
		Name:  "after",
		Value: time.Format(BigQueryTimestampFormat),
	}
}

func (qb *BigQueryTableQueryBuilder) Before(time *time.Time) {
	qb.parameters["before"] = bigquery.QueryParameter{
		Name:  "before",
		Value: time.Format(BigQueryTimestampFormat),
	}
}

func (qb *BigQueryTableQueryBuilder) Build() (*bigquery.Query, error) {
	var sb strings.Builder

	sb.WriteString("SELECT ")
	sb.WriteString("aead.decrypt_bytes(FROM_BASE64(@encryptionKey), encrypted_payload, b'')")
	sb.WriteString(", timestamp, log_message_id\n")

	sb.WriteString("FROM `")
	sb.WriteString(strings.Join([]string{qb.table.ProjectID, qb.table.DatasetID, qb.table.TableID}, "."))
	sb.WriteString("`\n")

	sb.WriteString("WHERE log_id = @logID\n")

	if _, ok := qb.parameters["startAt"]; ok {
		sb.WriteString("AND timestamp >= TIMESTAMP(@startAt)\n")
	}

	if _, ok := qb.parameters["endAt"]; ok {
		sb.WriteString("AND timestamp <= TIMESTAMP(@endAt)\n")
	}

	if _, ok := qb.parameters["after"]; ok {
		sb.WriteString("AND timestamp > TIMESTAMP(@after)\n")
	}

	if _, ok := qb.parameters["before"]; ok {
		sb.WriteString("AND timestamp < TIMESTAMP(@before)\n")
	}

	sb.WriteString("ORDER BY timestamp\n")

	if qb.client != nil {
		q := qb.client.Query(sb.String())

		for _, value := range qb.parameters {
			q.Parameters = append(q.Parameters, value)
		}

		return q, nil
	}

	return nil, nil
}

func NewBigQueryTableQueryBuilder() *BigQueryTableQueryBuilder {
	return &BigQueryTableQueryBuilder{
		parameters: make(map[string]bigquery.QueryParameter),
	}
}

type LogMessage struct {
	LogID            string
	LogMessageID     string
	Timestamp        time.Time
	EncryptedPayload []byte
}

func (lm *LogMessage) Save() (map[string]bigquery.Value, string, error) {
	return map[string]bigquery.Value{
		"log_id":            lm.LogID,
		"log_message_id":    lm.LogMessageID,
		"timestamp":         lm.Timestamp,
		"encrypted_payload": lm.EncryptedPayload,
	}, "", nil
}
