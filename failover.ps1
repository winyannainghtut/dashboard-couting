# Manual Failover Script for PostgreSQL Cluster

# 1. Stop the current master (simulating failure)
docker stop dashboard-couting-pg-master-1

# 2. Promote a standby (e.g., slave-1) to be the new primary
Write-Host "Promoting pg-slave-1 to Primary..."
docker exec dashboard-couting-pg-slave-1-1 pg_ctl promote

# Note: The application is configured with a multi-host connection string
# (target_session_attrs=read-write), so it will automatically detecting
# the new primary (slave-1) and resume writes.
