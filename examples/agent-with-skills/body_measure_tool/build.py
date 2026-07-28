#!/usr/bin/env python
# -*- coding:utf-8 -*-
"""
打包脚本（使用 .spec 文件方式）
第一次运行会生成 .spec 文件并自动打包，
后续可直接用 python -m PyInstaller 包名.spec 复用

用法：
  python build.py          # 首次：生成spec+打包；后续：直接按spec打包
  python build.py clean    # 清理 build/ dist/ 和 .spec 文件后重新打包
"""
import PyInstaller.__main__
import os, sys, shutil

os.chdir(os.path.dirname(os.path.abspath(__file__)))

spec_name = '身体测量数据转换工具.spec'
project_root = os.path.dirname(os.path.abspath(__file__))

# 清理模式
if len(sys.argv) > 1 and sys.argv[1] == 'clean':
    for d in ['build', 'dist']:
        p = os.path.join(project_root, d)
        if os.path.exists(p):
            shutil.rmtree(p)
            print(f'[清理] 删除目录: {d}/')
    for f in os.listdir(project_root):
        if f.endswith('.spec') or f.endswith('.exe'):
            os.remove(os.path.join(project_root, f))
            print(f'[清理] 删除文件: {f}')
    print('[OK] 清理完成，请重新运行 python build.py')
    sys.exit(0)

if not os.path.exists(spec_name):
    print(f'[构建] .spec 文件不存在，正在生成...')
    PyInstaller.__main__.run([
        'main.py',
        '--name=身体测量数据转换工具',
        '--onefile',
        '--add-data=body_config.json;.',
        '--noconsole',
        '--specpath=.',
    ])
    print(f'[OK] .spec 文件已生成: {spec_name}')
else:
    print(f'[构建] 使用已有 .spec 文件打包...')
    PyInstaller.__main__.run([
        spec_name,
        '--clean',
    ])
    print(f'[OK] 打包完成！exe 位于 dist/身体测量数据转换工具.exe')
