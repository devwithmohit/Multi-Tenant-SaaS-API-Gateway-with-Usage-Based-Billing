#!/bin/bash

set -e

BASE_DIR="infra"

echo "📁 Verifying infra structure..."

# Create directories
mkdir -p \
$BASE_DIR/k8s/gateway \
$BASE_DIR/k8s/usage-aggregator \
$BASE_DIR/k8s/dashboard-api \
$BASE_DIR/k8s/billing-engine \
$BASE_DIR/monitoring/prometheus \
$BASE_DIR/monitoring/grafana/dashboards \
$BASE_DIR/monitoring/grafana/dashboard-files \
$BASE_DIR/monitoring/grafana/datasources \
$BASE_DIR/monitoring/alertmanager

# Create files (only if not exist)
create_file_if_missing() {
  if [ ! -f "$1" ]; then
    touch "$1"
    echo "✅ Created: $1"
  else
    echo "⏭️ Exists: $1"
  fi
}

# --- k8s root ---
create_file_if_missing $BASE_DIR/k8s/namespace.yaml
create_file_if_missing $BASE_DIR/k8s/ingress.yaml
create_file_if_missing $BASE_DIR/k8s/secrets.yaml.template
create_file_if_missing $BASE_DIR/k8s/README.md

# --- gateway ---
create_file_if_missing $BASE_DIR/k8s/gateway/deployment.yaml
create_file_if_missing $BASE_DIR/k8s/gateway/service.yaml
create_file_if_missing $BASE_DIR/k8s/gateway/hpa.yaml
create_file_if_missing $BASE_DIR/k8s/gateway/configmap.yaml

# --- usage-aggregator ---
create_file_if_missing $BASE_DIR/k8s/usage-aggregator/deployment.yaml

# --- dashboard-api ---
create_file_if_missing $BASE_DIR/k8s/dashboard-api/deployment.yaml
create_file_if_missing $BASE_DIR/k8s/dashboard-api/service.yaml

# --- billing-engine ---
create_file_if_missing $BASE_DIR/k8s/billing-engine/cronjob.yaml

# --- monitoring root ---
create_file_if_missing $BASE_DIR/monitoring/docker-compose.yml
create_file_if_missing $BASE_DIR/monitoring/README.md
create_file_if_missing $BASE_DIR/monitoring/ALERTING.md

# --- prometheus ---
create_file_if_missing $BASE_DIR/monitoring/prometheus/prometheus.yml
create_file_if_missing $BASE_DIR/monitoring/prometheus/alerts.yml

# --- grafana ---
create_file_if_missing $BASE_DIR/monitoring/grafana/dashboards/dashboard.yml
create_file_if_missing $BASE_DIR/monitoring/grafana/dashboard-files/gateway-performance.json
create_file_if_missing $BASE_DIR/monitoring/grafana/dashboard-files/billing-revenue.json
create_file_if_missing $BASE_DIR/monitoring/grafana/datasources/datasource.yml

# --- alertmanager ---
create_file_if_missing $BASE_DIR/monitoring/alertmanager/config.yml

echo "🎯 Infra setup complete."
