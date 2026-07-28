#!/usr/bin/env python
# -*- coding:utf-8 -*-
"""
身高-胸围/腰围 范围匹配转换工具 (PyQt5)

功能：
    Tab1-数据转换：输入个人的身高、胸围、腰围及性别，自动匹配配置规则输出配置值。
    Tab2-配置管理：分男女配置身高+胸围/身高+腰围的范围规则。

规则匹配逻辑：
    按顺序遍历，找到第一条满足 性别+身高+围度 都在范围内的规则。
    数据格式：[性别, 身高下限, 身高上限, 围度下限, 围度上限, 配置值]
"""
import sys
import json
import os
from PyQt5.QtWidgets import (QApplication, QMainWindow, QWidget, QGridLayout,
                             QVBoxLayout, QHBoxLayout, QLabel, QLineEdit,
                             QPushButton, QTableWidget, QTableWidgetItem,
                             QHeaderView, QMessageBox, QGroupBox, QFrame,
                             QAbstractItemView, QDoubleSpinBox, QTabWidget,
                             QDialog, QComboBox, QRadioButton, QButtonGroup)
from PyQt5.QtCore import Qt, pyqtSignal
from PyQt5.QtGui import QFont, QIcon

# ============================================================
# 常量
# ============================================================
CONFIG_FILE = 'body_config.json'

# 配置管理表头
CONFIG_HEADERS_CHEST = ['性别', '身高下限', '身高上限', '胸围下限', '胸围上限', '配置胸围值']
CONFIG_HEADERS_WAIST = ['性别', '身高下限', '身高上限', '腰围下限', '腰围上限', '配置腰围值']

# ============================================================
# 公共函数
# ============================================================
def load_config():
    """读取配置文件，返回 {'chest': [...], 'waist': [...]}"""
    if not os.path.exists(CONFIG_FILE):
        return {'chest': [], 'waist': []}
    with open(CONFIG_FILE, 'r', encoding='utf-8') as f:
        return json.load(f)


def save_config(data):
    """保存配置文件"""
    with open(CONFIG_FILE, 'w', encoding='utf-8') as f:
        json.dump(data, f, ensure_ascii=False, indent=2)


def match_value(gender, height, measure, rules):
    """
    匹配规则：遍历 rules，找到第一条满足 性别+身高+围度 都在范围内的规则。
    rules: [(gender, h_min, h_max, m_min, m_max, value), ...]
    返回匹配的 value，未匹配返回 None。
    """
    for g, h_min, h_max, m_min, m_max, value in rules:
        if g == gender and h_min <= height <= h_max and m_min <= measure <= m_max:
            return value
    return None


# ============================================================
# 配置编辑对话框
# ============================================================
class RuleEditDialog(QDialog):
    """单条规则编辑弹窗"""

    def __init__(self, title, headers, values=None, parent=None):
        """
        :param title: 对话框标题
        :param headers: 字段名称列表（包含'性别'）
        :param values: 编辑时传入的现有值列表，新增时 None
        """
        super().__init__(parent)
        self.setWindowTitle(title)
        self.setFixedSize(480, 320)
        self.setStyleSheet("""
            QWidget { background-color: white; font: 12pt "微软雅黑"; }
            QLineEdit, QDoubleSpinBox, QComboBox {
                border: 1px solid lightgray;
                padding: 4px;
                border-radius: 3px;
            }
            QLineEdit:focus, QDoubleSpinBox:focus {
                border: 1px solid #63acfb;
            }
            QPushButton {
                background-color: #57bd6a;
                color: #f9ffff;
                font-size: 16px;
                font-weight: bold;
                border-radius: 5px;
                padding: 6px 20px;
            }
            QPushButton:pressed {
                background-color: #4eaa5f;
            }
            QPushButton#btnCancel {
                background-color: rgb(184, 184, 184);
            }
            QPushButton#btnCancel:pressed {
                background-color: rgb(207, 207, 207);
            }
        """)
        self.values = values
        self.result_values = None

        layout = QVBoxLayout(self)
        layout.setSpacing(12)

        form_layout = QGridLayout()
        form_layout.setVerticalSpacing(10)

        # 性别（第一列使用下拉框）
        lbl_gender = QLabel('*性别')
        self.cb_gender = QComboBox()
        self.cb_gender.addItems(['男', '女'])
        if values:
            self.cb_gender.setCurrentText(values[0])
        form_layout.addWidget(lbl_gender, 0, 0, Qt.AlignRight)
        form_layout.addWidget(self.cb_gender, 0, 1)

        # 其余字段
        for i in range(1, len(headers)):
            label_text = headers[i]
            lbl = QLabel(f'*{label_text}')
            sb = QDoubleSpinBox()
            sb.setRange(0, 99999)
            sb.setDecimals(1)
            sb.setSingleStep(1)
            if values:
                sb.setValue(values[i])
            form_layout.addWidget(lbl, i, 0, Qt.AlignRight)
            form_layout.addWidget(sb, i, 1)
            setattr(self, f'spin_{i}', sb)

        layout.addLayout(form_layout)
        layout.addStretch()

        btn_layout = QHBoxLayout()
        btn_layout.addStretch()

        self.btn_cancel = QPushButton('取消')
        self.btn_cancel.setObjectName('btnCancel')
        self.btn_cancel.setFixedWidth(80)
        self.btn_cancel.clicked.connect(self.reject)

        self.btn_ok = QPushButton('确定')
        self.btn_ok.setFixedWidth(80)
        self.btn_ok.clicked.connect(self.on_confirm)

        btn_layout.addWidget(self.btn_cancel)
        btn_layout.addWidget(self.btn_ok)
        layout.addLayout(btn_layout)

    def on_confirm(self):
        gender = self.cb_gender.currentText()
        h_min = getattr(self, 'spin_1').value()
        h_max = getattr(self, 'spin_2').value()
        m_min = getattr(self, 'spin_3').value()
        m_max = getattr(self, 'spin_4').value()
        value = getattr(self, 'spin_5').value()

        if h_min > h_max:
            QMessageBox.warning(self, '提示', '身高下限不能大于身高上限')
            return
        if m_min > m_max:
            QMessageBox.warning(self, '提示', '围度下限不能大于围度上限')
            return
        self.result_values = [gender, h_min, h_max, m_min, m_max, value]
        self.accept()

    def get_values(self):
        return self.result_values


# ============================================================
# 配置管理页面（统一管理胸围和腰围，含男女）
# ============================================================
class ConfigPage(QWidget):
    """配置管理 Tab：胸围配置表 + 腰围配置表，均分男女"""

    def __init__(self, parent=None):
        super().__init__(parent)
        self.chest_data = []   # [(gender,h_min,h_max,m_min,m_max,value), ...]
        self.waist_data = []   # [(gender,h_min,h_max,m_min,m_max,value), ...]
        self.init_ui()
        self.load_data()

    def init_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(5, 5, 5, 5)

        # ========== 胸围配置区域 ==========
        chest_group = QGroupBox('胸围配置规则')
        chest_group.setStyleSheet("""
            QGroupBox {
                font: 13pt "微软雅黑";
                font-weight: bold;
                border: 1px solid lightgray;
                border-radius: 6px;
                margin-top: 10px;
                padding-top: 18px;
            }
            QGroupBox::title {
                subcontrol-origin: margin;
                left: 12px;
                padding: 0 6px;
            }
        """)
        chest_layout = QVBoxLayout(chest_group)

        # 按钮行
        chest_btn_layout = QHBoxLayout()
        self.btn_chest_add = QPushButton('新增胸围规则')
        self.btn_chest_edit = QPushButton('编辑选中')
        self.btn_chest_delete = QPushButton('删除选中')
        for btn in (self.btn_chest_add, self.btn_chest_edit):
            btn.setStyleSheet("""
                QPushButton {
                    background-color: #57bd6a;
                    color: #f9ffff;
                    font-size: 13px;
                    font-weight: bold;
                    border-radius: 5px;
                    padding: 5px 14px;
                }
                QPushButton:pressed { background-color: #4eaa5f; }
            """)
        self.btn_chest_delete.setStyleSheet("""
            QPushButton {
                background-color: #e74c3c;
                color: #f9ffff;
                font-size: 13px;
                font-weight: bold;
                border-radius: 5px;
                padding: 5px 14px;
            }
            QPushButton:pressed { background-color: #c0392b; }
        """)
        chest_btn_layout.addWidget(self.btn_chest_add)
        chest_btn_layout.addWidget(self.btn_chest_edit)
        chest_btn_layout.addWidget(self.btn_chest_delete)
        chest_btn_layout.addStretch()
        chest_layout.addLayout(chest_btn_layout)

        # 胸围表格
        self.table_chest = QTableWidget()
        self.table_chest.setColumnCount(len(CONFIG_HEADERS_CHEST))
        self.table_chest.setHorizontalHeaderLabels(CONFIG_HEADERS_CHEST)
        self.table_chest.horizontalHeader().setStretchLastSection(True)
        self.table_chest.horizontalHeader().setSectionResizeMode(QHeaderView.Stretch)
        self.table_chest.setSelectionBehavior(QAbstractItemView.SelectRows)
        self.table_chest.setSelectionMode(QAbstractItemView.SingleSelection)
        self.table_chest.setEditTriggers(QAbstractItemView.NoEditTriggers)
        self.table_chest.verticalHeader().setVisible(False)
        self.table_chest.setMaximumHeight(220)
        self.table_chest.setStyleSheet("""
            QTableWidget {
                border: 1px solid lightgray;
                gridline-color: #e0e0e0;
                font: 10.5pt "微软雅黑";
            }
            QHeaderView::section {
                background-color: #f5f5f5;
                border: 1px solid lightgray;
                padding: 4px;
                font-weight: bold;
                font-size: 10.5pt;
            }
            QTableWidget::item:selected {
                background-color: #d4e6ff;
                color: black;
            }
        """)
        chest_layout.addWidget(self.table_chest)

        # 信号
        self.btn_chest_add.clicked.connect(lambda: self.on_add('chest'))
        self.btn_chest_edit.clicked.connect(lambda: self.on_edit('chest'))
        self.btn_chest_delete.clicked.connect(lambda: self.on_delete('chest'))
        self.table_chest.doubleClicked.connect(lambda: self.on_edit('chest'))

        layout.addWidget(chest_group)

        # ========== 腰围配置区域 ==========
        waist_group = QGroupBox('腰围配置规则')
        waist_group.setStyleSheet("""
            QGroupBox {
                font: 13pt "微软雅黑";
                font-weight: bold;
                border: 1px solid lightgray;
                border-radius: 6px;
                margin-top: 10px;
                padding-top: 18px;
            }
            QGroupBox::title {
                subcontrol-origin: margin;
                left: 12px;
                padding: 0 6px;
            }
        """)
        waist_layout = QVBoxLayout(waist_group)

        # 按钮行
        waist_btn_layout = QHBoxLayout()
        self.btn_waist_add = QPushButton('新增腰围规则')
        self.btn_waist_edit = QPushButton('编辑选中')
        self.btn_waist_delete = QPushButton('删除选中')
        for btn in (self.btn_waist_add, self.btn_waist_edit):
            btn.setStyleSheet("""
                QPushButton {
                    background-color: #57bd6a;
                    color: #f9ffff;
                    font-size: 13px;
                    font-weight: bold;
                    border-radius: 5px;
                    padding: 5px 14px;
                }
                QPushButton:pressed { background-color: #4eaa5f; }
            """)
        self.btn_waist_delete.setStyleSheet("""
            QPushButton {
                background-color: #e74c3c;
                color: #f9ffff;
                font-size: 13px;
                font-weight: bold;
                border-radius: 5px;
                padding: 5px 14px;
            }
            QPushButton:pressed { background-color: #c0392b; }
        """)
        waist_btn_layout.addWidget(self.btn_waist_add)
        waist_btn_layout.addWidget(self.btn_waist_edit)
        waist_btn_layout.addWidget(self.btn_waist_delete)
        waist_btn_layout.addStretch()
        waist_layout.addLayout(waist_btn_layout)

        # 腰围表格
        self.table_waist = QTableWidget()
        self.table_waist.setColumnCount(len(CONFIG_HEADERS_WAIST))
        self.table_waist.setHorizontalHeaderLabels(CONFIG_HEADERS_WAIST)
        self.table_waist.horizontalHeader().setStretchLastSection(True)
        self.table_waist.horizontalHeader().setSectionResizeMode(QHeaderView.Stretch)
        self.table_waist.setSelectionBehavior(QAbstractItemView.SelectRows)
        self.table_waist.setSelectionMode(QAbstractItemView.SingleSelection)
        self.table_waist.setEditTriggers(QAbstractItemView.NoEditTriggers)
        self.table_waist.verticalHeader().setVisible(False)
        self.table_waist.setMaximumHeight(220)
        self.table_waist.setStyleSheet("""
            QTableWidget {
                border: 1px solid lightgray;
                gridline-color: #e0e0e0;
                font: 10.5pt "微软雅黑";
            }
            QHeaderView::section {
                background-color: #f5f5f5;
                border: 1px solid lightgray;
                padding: 4px;
                font-weight: bold;
                font-size: 10.5pt;
            }
            QTableWidget::item:selected {
                background-color: #d4e6ff;
                color: black;
            }
        """)
        waist_layout.addWidget(self.table_waist)

        # 信号
        self.btn_waist_add.clicked.connect(lambda: self.on_add('waist'))
        self.btn_waist_edit.clicked.connect(lambda: self.on_edit('waist'))
        self.btn_waist_delete.clicked.connect(lambda: self.on_delete('waist'))
        self.table_waist.doubleClicked.connect(lambda: self.on_edit('waist'))

        layout.addWidget(waist_group)
        layout.addStretch()

    def load_data(self):
        """从 JSON 加载数据"""
        config = load_config()
        self.chest_data = [tuple(item) for item in config.get('chest', [])]
        self.waist_data = [tuple(item) for item in config.get('waist', [])]
        self._refresh_table('chest')
        self._refresh_table('waist')

    def _refresh_table(self, rule_type):
        """刷新表格"""
        if rule_type == 'chest':
            table = self.table_chest
            data = self.chest_data
        else:
            table = self.table_waist
            data = self.waist_data

        table.setRowCount(len(data))
        for row_idx, row_data in enumerate(data):
            for col_idx, val in enumerate(row_data):
                item = QTableWidgetItem(str(val))
                item.setTextAlignment(Qt.AlignCenter)
                table.setItem(row_idx, col_idx, item)

    def _save_data(self, rule_type):
        """保存到 JSON"""
        config = load_config()
        if rule_type == 'chest':
            config['chest'] = [list(t) for t in self.chest_data]
        else:
            config['waist'] = [list(t) for t in self.waist_data]
        save_config(config)

    def _get_data(self, rule_type):
        if rule_type == 'chest':
            return self.chest_data
        return self.waist_data

    def _set_data(self, rule_type, data):
        if rule_type == 'chest':
            self.chest_data = data
        else:
            self.waist_data = data

    def on_add(self, rule_type):
        name = '胸围' if rule_type == 'chest' else '腰围'
        headers = CONFIG_HEADERS_CHEST if rule_type == 'chest' else CONFIG_HEADERS_WAIST
        dialog = RuleEditDialog(f'新增{name}规则', headers)
        if dialog.exec_() == QDialog.Accepted:
            values = dialog.get_values()
            data = self._get_data(rule_type)
            data.append(tuple(values))
            self._set_data(rule_type, data)
            self._refresh_table(rule_type)
            self._save_data(rule_type)

    def on_edit(self, rule_type):
        table = self.table_chest if rule_type == 'chest' else self.table_waist
        row = table.currentRow()
        if row < 0:
            QMessageBox.information(self, '提示', '请先选中一条规则')
            return
        data = self._get_data(rule_type)
        old_values = list(data[row])
        name = '胸围' if rule_type == 'chest' else '腰围'
        headers = CONFIG_HEADERS_CHEST if rule_type == 'chest' else CONFIG_HEADERS_WAIST
        dialog = RuleEditDialog(f'编辑{name}规则', headers, values=old_values)
        if dialog.exec_() == QDialog.Accepted:
            new_values = dialog.get_values()
            data[row] = tuple(new_values)
            self._set_data(rule_type, data)
            self._refresh_table(rule_type)
            self._save_data(rule_type)

    def on_delete(self, rule_type):
        table = self.table_chest if rule_type == 'chest' else self.table_waist
        row = table.currentRow()
        if row < 0:
            QMessageBox.information(self, '提示', '请先选中一条规则')
            return
        reply = QMessageBox.question(
            self, '确认删除', '确定要删除选中的规则吗？',
            QMessageBox.Yes | QMessageBox.No
        )
        if reply == QMessageBox.Yes:
            data = self._get_data(rule_type)
            data.pop(row)
            self._set_data(rule_type, data)
            self._refresh_table(rule_type)
            self._save_data(rule_type)

    def get_chest_rules(self):
        return self.chest_data

    def get_waist_rules(self):
        return self.waist_data


# ============================================================
# 转换计算页面（Tab1）
# ============================================================
class ConvertPage(QWidget):
    """输入性别、身高、胸围、腰围，计算配置值"""

    def __init__(self, parent=None):
        super().__init__(parent)
        self.chest_rules = []
        self.waist_rules = []
        self.init_ui()

    def init_ui(self):
        layout = QVBoxLayout(self)
        layout.setSpacing(15)
        layout.setContentsMargins(20, 20, 20, 20)

        # ---- 标题 ----
        title = QLabel('身体测量数据转换')
        title.setStyleSheet("""
            font: 20pt "微软雅黑";
            font-weight: bold;
            color: #2c3e50;
            padding: 10px 0;
        """)
        title.setAlignment(Qt.AlignCenter)
        layout.addWidget(title)

        # ---- 输入区域 ----
        input_group = QGroupBox('输入个人数据')
        input_group.setStyleSheet("""
            QGroupBox {
                font: 13pt "微软雅黑";
                font-weight: bold;
                border: 1px solid lightgray;
                border-radius: 6px;
                margin-top: 10px;
                padding-top: 18px;
            }
            QGroupBox::title {
                subcontrol-origin: margin;
                left: 12px;
                padding: 0 6px;
            }
        """)
        input_layout = QGridLayout(input_group)
        input_layout.setVerticalSpacing(12)
        input_layout.setHorizontalSpacing(10)

        # 性别
        lbl_gender = QLabel('性别：')
        lbl_gender.setStyleSheet("font: 12pt '微软雅黑';")

        gender_group = QButtonGroup(self)
        self.rb_male = QRadioButton('男')
        self.rb_female = QRadioButton('女')
        self.rb_male.setStyleSheet("font: 12pt '微软雅黑';")
        self.rb_female.setStyleSheet("font: 12pt '微软雅黑';")
        self.rb_male.setChecked(True)
        gender_group.addButton(self.rb_male)
        gender_group.addButton(self.rb_female)

        gender_widget = QWidget()
        gender_layout = QHBoxLayout(gender_widget)
        gender_layout.setContentsMargins(0, 0, 0, 0)
        gender_layout.addWidget(self.rb_male)
        gender_layout.addWidget(self.rb_female)
        gender_layout.addStretch()

        input_layout.addWidget(lbl_gender, 0, 0, Qt.AlignRight)
        input_layout.addWidget(gender_widget, 0, 1)

        # 其余字段
        fields = [
            ('身高 (cm)：', 'input_height', 1),
            ('胸围 (cm)：', 'input_chest', 2),
            ('腰围 (cm)：', 'input_waist', 3),
        ]

        self.inputs = {}
        for label_text, attr, row in fields:
            lbl = QLabel(label_text)
            lbl.setStyleSheet("font: 12pt '微软雅黑';")
            le = QLineEdit()
            le.setPlaceholderText('请输入数值')
            le.setStyleSheet("""
                QLineEdit {
                    border: 1px solid lightgray;
                    padding: 6px;
                    border-radius: 4px;
                    font: 12pt '微软雅黑';
                }
                QLineEdit:focus {
                    border: 1px solid #63acfb;
                }
            """)
            input_layout.addWidget(lbl, row, 0, Qt.AlignRight)
            input_layout.addWidget(le, row, 1)
            self.inputs[attr] = le

        # ---- 转换按钮 ----
        btn_convert_layout = QHBoxLayout()
        self.btn_convert = QPushButton('开始转换')
        self.btn_convert.setStyleSheet("""
            QPushButton {
                background-color: #57bd6a;
                color: #f9ffff;
                font-size: 18px;
                font-weight: bold;
                border-radius: 6px;
                padding: 10px 30px;
            }
            QPushButton:pressed {
                background-color: #4eaa5f;
            }
        """)
        self.btn_convert.setFixedHeight(45)

        self.btn_clear = QPushButton('清除')
        self.btn_clear.setStyleSheet("""
            QPushButton {
                background-color: rgb(184, 184, 184);
                color: #f9ffff;
                font-size: 14px;
                font-weight: bold;
                border-radius: 5px;
                padding: 6px 20px;
            }
            QPushButton:pressed {
                background-color: rgb(207, 207, 207);
            }
        """)

        btn_convert_layout.addStretch()
        btn_convert_layout.addWidget(self.btn_convert)
        btn_convert_layout.addWidget(self.btn_clear)
        btn_convert_layout.addStretch()
        input_layout.addLayout(btn_convert_layout, 4, 0, 1, 2)

        layout.addWidget(input_group)

        # ---- 结果区域 ----
        result_group = QGroupBox('转换结果')
        result_group.setStyleSheet("""
            QGroupBox {
                font: 13pt "微软雅黑";
                font-weight: bold;
                border: 1px solid lightgray;
                border-radius: 6px;
                margin-top: 10px;
                padding-top: 18px;
            }
            QGroupBox::title {
                subcontrol-origin: margin;
                left: 12px;
                padding: 0 6px;
            }
        """)
        result_layout = QGridLayout(result_group)
        result_layout.setVerticalSpacing(10)

        self.result_chest = QLabel('--')
        self.result_chest.setStyleSheet("""
            font: 16pt "微软雅黑";
            font-weight: bold;
            color: #2c3e50;
            background-color: #f0f8ff;
            border: 1px solid #b8d4f0;
            border-radius: 5px;
            padding: 8px;
        """)
        self.result_chest.setAlignment(Qt.AlignCenter)

        self.result_waist = QLabel('--')
        self.result_waist.setStyleSheet("""
            font: 16pt "微软雅黑";
            font-weight: bold;
            color: #2c3e50;
            background-color: #f0f8ff;
            border: 1px solid #b8d4f0;
            border-radius: 5px;
            padding: 8px;
        """)
        self.result_waist.setAlignment(Qt.AlignCenter)

        result_layout.addWidget(QLabel('配置胸围 (cm)：'), 0, 0, Qt.AlignRight)
        result_layout.addWidget(self.result_chest, 0, 1)
        result_layout.addWidget(QLabel('配置腰围 (cm)：'), 1, 0, Qt.AlignRight)
        result_layout.addWidget(self.result_waist, 1, 1)

        self.label_hint = QLabel('')
        self.label_hint.setStyleSheet("font: 10pt '微软雅黑'; color: gray;")
        self.label_hint.setAlignment(Qt.AlignCenter)
        result_layout.addWidget(self.label_hint, 2, 0, 1, 2)

        layout.addWidget(result_group)
        layout.addStretch()

        # ---- 信号 ----
        self.btn_convert.clicked.connect(self.on_convert)
        self.btn_clear.clicked.connect(self.on_clear)

    def update_rules(self, chest_rules, waist_rules):
        """由主窗口调用，更新规则数据"""
        self.chest_rules = chest_rules
        self.waist_rules = waist_rules

    def on_convert(self):
        """执行转换"""
        gender = '男' if self.rb_male.isChecked() else '女'

        try:
            height = float(self.inputs['input_height'].text())
            chest = float(self.inputs['input_chest'].text())
            waist = float(self.inputs['input_waist'].text())
        except ValueError:
            QMessageBox.warning(self, '输入错误', '请正确填写身高、胸围、腰围数值')
            return

        # 匹配
        matched_chest = match_value(gender, height, chest, self.chest_rules)
        matched_waist = match_value(gender, height, waist, self.waist_rules)

        # 显示结果
        def set_result(label, value):
            if value is not None:
                label.setText(f'{value:.1f}')
                label.setStyleSheet("""
                    font: 16pt "微软雅黑"; font-weight: bold;
                    color: #2c3e50; background-color: #e8f8e8;
                    border: 1px solid #a0d8a0; border-radius: 5px; padding: 8px;
                """)
            else:
                label.setText('未匹配')
                label.setStyleSheet("""
                    font: 16pt "微软雅黑"; font-weight: bold;
                    color: #e74c3c; background-color: #fde8e8;
                    border: 1px solid #f0b0b0; border-radius: 5px; padding: 8px;
                """)

        set_result(self.result_chest, matched_chest)
        set_result(self.result_waist, matched_waist)

        hints = []
        if matched_chest is None:
            hints.append('胸围')
        if matched_waist is None:
            hints.append('腰围')
        if hints:
            self.label_hint.setText(f'未找到匹配的{"、".join(hints)}配置规则，请检查配置')
            self.label_hint.setStyleSheet("font: 10pt '微软雅黑'; color: #e74c3c;")
        else:
            self.label_hint.setText('匹配成功 ✓')
            self.label_hint.setStyleSheet("font: 10pt '微软雅黑'; color: #27ae60;")

    def on_clear(self):
        """清除输入和结果"""
        for le in self.inputs.values():
            le.clear()
        self.result_chest.setText('--')
        self.result_chest.setStyleSheet("""
            font: 16pt "微软雅黑"; font-weight: bold;
            color: #2c3e50; background-color: #f0f8ff;
            border: 1px solid #b8d4f0; border-radius: 5px; padding: 8px;
        """)
        self.result_waist.setText('--')
        self.result_waist.setStyleSheet("""
            font: 16pt "微软雅黑"; font-weight: bold;
            color: #2c3e50; background-color: #f0f8ff;
            border: 1px solid #b8d4f0; border-radius: 5px; padding: 8px;
        """)
        self.label_hint.setText('')


# ============================================================
# 主窗口
# ============================================================
class MainWindow(QMainWindow):
    """主窗口：Tab 布局"""

    def __init__(self):
        super().__init__()
        self.setWindowTitle('身体测量数据转换工具')
        self.setMinimumSize(860, 680)
        self.setStyleSheet("""
            QMainWindow { background-color: white; }
            QWidget { font: 12pt "微软雅黑"; }
        """)

        central = QWidget()
        self.setCentralWidget(central)
        main_layout = QVBoxLayout(central)
        main_layout.setContentsMargins(0, 0, 0, 0)

        # ---- Tab 切换 ----
        self.tabs = QTabWidget()
        self.tabs.setStyleSheet("""
            QTabWidget::pane {
                border: 1px solid lightgray;
                background: white;
            }
            QTabBar::tab {
                font: 12pt "微软雅黑";
                padding: 8px 20px;
                border: 1px solid lightgray;
                border-bottom: none;
                border-top-left-radius: 6px;
                border-top-right-radius: 6px;
                background: #f5f5f5;
                margin-right: 2px;
            }
            QTabBar::tab:selected {
                background: white;
                border-bottom: 2px solid #57bd6a;
                font-weight: bold;
            }
            QTabBar::tab:hover:!selected {
                background: #e8e8e8;
            }
        """)

        # 数据转换放第一位，配置管理放第二位
        self.tab_convert = ConvertPage()
        self.tab_config = ConfigPage()

        self.tabs.addTab(self.tab_convert, '数据转换')
        self.tabs.addTab(self.tab_config, '配置管理')

        main_layout.addWidget(self.tabs)

        # 切换 Tab 时同步规则
        self.tabs.currentChanged.connect(self.on_tab_changed)

        # 初始同步
        self.sync_rules_to_convert()

    def on_tab_changed(self, index):
        """切换到数据转换 Tab 时同步规则"""
        if index == 0:
            self.sync_rules_to_convert()

    def sync_rules_to_convert(self):
        """将配置页的规则同步到转换页"""
        self.tab_convert.update_rules(
            self.tab_config.get_chest_rules(),
            self.tab_config.get_waist_rules(),
        )


# ============================================================
# 入口
# ============================================================
if __name__ == '__main__':
    app = QApplication(sys.argv)
    w = MainWindow()
    w.show()
    sys.exit(app.exec())
