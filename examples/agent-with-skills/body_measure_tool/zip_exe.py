#!/usr/bin/env python
# -*- coding:utf-8 -*-
"""
压缩已打包的 exe 文件和相关依赖为 zip
用法：先运行 python build.py，再运行 python zip_exe.py
"""
import shutil, os

os.chdir(os.path.dirname(os.path.abspath(__file__)))

# 找 dist 下的 .exe 文件
exe_files = [f for f in os.listdir('dist') if f.endswith('.exe')]
if not exe_files:
    print('错误: dist 目录下找不到 exe 文件')
    exit(1)

exe_name = exe_files[0].replace('.exe', '')  # "身体测量数据转换工具"
output_zip = exe_name + '.zip'

if os.path.exists(output_zip):
    os.remove(output_zip)

shutil.make_archive(exe_name, 'zip', 'dist')
print(f'打包完成: {output_zip}')
