@echo off
cd /d %~dp0..
controller\controller.exe > controller.log 2>&1
