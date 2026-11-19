# NOTICE — dissect

**dissect** — 本地游戏解包与逆向分析工具（Ren'Py / Unity）
A local unpacking and reverse-engineering analysis tool for Ren'Py and Unity games.

Copyright (c) 2026 Ma0de and contributors

本程序是自由软件：你可以依据自由软件基金会发布的 **GNU 通用公共许可证第 3 版（GPL-3.0）** 的条款再分发和/或修改它。本程序不附带任何担保；不提供适销性或特定用途适用性的暗示担保。详见 LICENSE 文件，或访问 <https://www.gnu.org/licenses/gpl-3.0.html>。

This program is free software: you can redistribute it and/or modify it under the terms of the **GNU General Public License as published by the Free Software Foundation, version 3** (GPL-3.0). This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the LICENSE file for details, or <https://www.gnu.org/licenses/gpl-3.0.html>.

SPDX-License-Identifier: GPL-3.0

---

## 第三方组件 Third-party components

本项目包含或依赖以下第三方代码。各组件的权利归其各自作者所有，本项目按其原许可证使用。

This project includes or depends on the following third-party code. All rights belong to the respective authors and are used under their original licenses.

| 组件 Component | 载体 Where | 许可证 License | 版权与来源 Copyright / Source |
| --- | --- | --- | --- |
| unrpyc（完整捆绑 bundled verbatim） | `vendor/unrpyc/` | MIT | Copyright (c) 2012-2024 Yuri K. Schlesner, CensoredUsername, Jackmcbarn — <https://github.com/CensoredUsername/unrpyc> |
| UnityPy（运行时依赖，未捆绑 runtime dependency, not bundled） | 由使用者经 `pip install UnityPy` 安装 | MIT | Copyright K0lb3 — <https://github.com/K0lb3/UnityPy> |
| Il2CppDumper（外部工具，未捆绑 external tool, not bundled） | 由使用者自行提供可执行文件 | 以该项目自身的许可证为准 use under its own license | <https://github.com/Perfare/Il2CppDumper> |
| Ren'Py 引擎（格式参考与 Web 运行时来源 reference format & web runtime） | 不随本项目分发 not distributed with this project | MIT / LGPL-2.1（部分组件 portions） | Tom Rothamel 等 — <https://www.renpy.org/l/license> |
| Web 前端框架 Web frontend framework | `web/package.json` | MIT（Next.js、React、Tailwind CSS、@xyflow/react 等，以 package-lock.json 为准） | 各自权利人 respective owners |

`vendor/unrpyc/` 目录保留其原始 MIT LICENSE 与 README 文件，未做结构性改动之外的移除。
The `vendor/unrpyc/` directory retains its original MIT LICENSE and README files.

---

## 商标与关联声明 Trademark and affiliation notice

**中文**：dissect 与 Ren'Py、Tom Rothamel、Unity Technologies 或任何游戏开发者/发行商均无隶属、合作或背书关系。"Ren'Py"、"Unity" 等名称仅为指示性使用，用于说明本工具所支持的文件格式，不构成商标性使用或任何形式的认可。各商标归其权利人所有。

**English**: dissect is not affiliated with, endorsed by, or sponsored by Ren'Py, Tom Rothamel, Unity Technologies, or any game developer or publisher. "Ren'Py", "Unity" and similar names are used solely for nominative reference to the file formats this tool supports. All trademarks are the property of their respective owners.

---

## 使用范围提示 Scope-of-use notice

本工具仅用于对使用者**合法拥有或已获权利人授权**的游戏文件进行本地分析、学习与研究（互操作性、存档、教学等）。本工具不提供、不支持任何针对数字版权保护措施（DRM）的破解或解密功能。使用者须自行确保其使用行为符合所在司法辖区法律及游戏权利人的条款。滥用本工具进行盗版传播的行为不属于本项目意图的用途，亦不为本项目所认可。

This tool is intended solely for local analysis, study, and research (interoperability, preservation, education) of game files the user **lawfully owns or is authorized to use**. The tool provides no functionality for breaking or decrypting digital rights management measures. Users are responsible for ensuring compliance with the laws of their jurisdiction and the terms of the game's rights holders. Using this tool to pirate or redistribute copyrighted content is not an intended use and is not endorsed by this project.
