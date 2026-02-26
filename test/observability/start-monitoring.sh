#!/bin/bash

# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

# Service Graph Monitoring Stack Startup Script

set -e

echo "Starting Service Graph Monitoring Stack..."

# Start the monitoring services
docker-compose up -d

echo ""
echo "Monitoring services started successfully!"
echo ""
echo "Access URLs:"
echo "  Jaeger (Tracing):     http://localhost:16686"
echo "  Prometheus (Metrics): http://localhost:9090"
echo "  Grafana (Dashboard):  http://localhost:3000 (admin/admin)"
echo ""
echo "To stop the services, run: docker-compose down"
echo "To view logs, run: docker-compose logs -f"