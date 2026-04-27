@echo off
cd /d %~dp0..
set CONTROLLER_URL=wss://127.0.0.1:8443/ws
set AGENT_TOKEN=dev-token
set AGENT_ID=test-machine-1
agent\agent.exe > agent.log 2>&1
