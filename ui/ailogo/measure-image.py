#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
measure-image.py —— 盲量图片工具(不靠肉眼/识图接口)

把图片当数字矩阵分析,输出:
  1. 尺寸 / 模式 / 是否透明背景
  2. 背景色(若不透明)
  3. 内容包围盒(去掉透明/背景留白后的真实画面区域,像素 + 百分比)
  4. 主要颜色(top)
  5. 横向 / 纵向"长条"候选(进度条、分隔线等)

用途:为"在原图上叠加动画"提供精确坐标和颜色,换图即跑即得。

用法:
  python3 measure-image.py <图片> [图片 ...]
  python3 measure-image.py "AI Arsenal 启动页 Loading.png"

依赖:Pillow  (pip install --user Pillow)
"""
import sys
from PIL import Image

BG_THRESH = 45          # 与背景的差异阈值
QUANT = 26              # 颜色量化粒度
SAMPLE_STEP_DIV = 420   # 采样密度(越大越稀疏越快)


def dist(a, b):
    return sum((x - y) ** 2 for x, y in zip(a, b)) ** 0.5


def analyze(path):
    img = Image.open(path)
    W, H = img.size
    rgba = img.convert("RGBA")
    px = rgba.load()
    step_x = max(1, W // SAMPLE_STEP_DIV)
    step_y = max(1, H // SAMPLE_STEP_DIV)

    # —— 1. 背景判定:四角 ——
    corners = [px[2, 2], px[W - 3, 2], px[2, H - 3], px[W - 3, H - 3]]
    transparent = any(c[3] < 16 for c in corners)

    def is_content(x, y):
        c = px[x, y]
        if c[3] <= 24:
            return False  # 透明
        if transparent:
            return True   # 透明底:任何不透明像素都算内容
        return dist(c[:3], bg) > BG_THRESH

    bg = None
    if not transparent:
        bg_cols = [c[:3] for c in corners]
        bg = tuple(sum(c[i] for c in bg_cols) // 4 for i in range(3))

    # —— 2. 内容包围盒 ——
    xs, ys = [], []
    for y in range(0, H, step_y):
        for x in range(0, W, step_x):
            if is_content(x, y):
                xs.append(x); ys.append(y)
    has_content = bool(xs)
    if has_content:
        cx1, cx2, cy1, cy2 = min(xs), max(xs), min(ys), max(ys)
        cw, ch = cx2 - cx1, cy2 - cy1

    print("=" * 64)
    print(f"图片: {path}")
    print(f"尺寸: {W} x {H}   模式: {img.mode}   透明背景: {'是' if transparent else '否'}")
    if bg:
        print(f"背景色 ≈ rgb{bg}")
    if has_content:
        print(f"内容包围盒: ({cx1},{cy1})-({cx2},{cy2})  实际内容 {cw}x{ch}")
        print(f"  占画布: left={cx1/W*100:.1f}%  top={cy1/H*100:.1f}%  "
              f"width={cw/W*100:.1f}%  height={ch/H*100:.1f}%")
        cover = (cw * ch) / (W * H) * 100
        print(f"  内容覆盖画布: {cover:.1f}%   {'⚠️ 大量留白/透明边距 → 直接显示会偏小' if cover < 60 else ''}")
    else:
        print("内容包围盒: (未检出内容)")

    # —— 3. 主要颜色 ——
    from collections import Counter
    cnt = Counter()
    for y in range(0, H, step_y):
        for x in range(0, W, step_x):
            c = px[x, y]
            if c[3] <= 24:
                continue
            col = c[:3]
            if bg and dist(col, bg) <= BG_THRESH:
                continue
            cnt[tuple(v // QUANT * QUANT for v in col)] += 1
    if cnt:
        print("主要颜色(量化) top6:")
        for col, n in cnt.most_common(6):
            print(f"  rgb{col}  样本 {n}")

    # —— 4. 横向/纵向长条候选(进度条等) ——
    def scan_bands(axis):
        # axis='h' 扫每行最长连续段; 'v' 扫每列
        rows = []
        if axis == 'h':
            outer, inner = range(0, H, max(1, H // 500)), range(W)
            get = lambda x, y: is_content(x, y)
        else:
            outer, inner = range(0, W, max(1, W // 500)), range(H)
            get = lambda x, y: is_content(x, y)
        for k in outer:
            run = best = 0; bs = 0; cur = 0
            for j in inner:
                x, y = (j, k) if axis == 'h' else (k, j)
                if get(x, y):
                    if run == 0: cur = j
                    run += 1
                    if run > best: best = run; bs = cur
                else:
                    run = 0
            if best > 250:
                rows.append((k, best, bs, bs + best))
        # 聚类
        bands = []; cur = []
        for k, best, s, e in rows:
            if cur and k - cur[-1][0] <= 4:
                cur.append((k, best, s, e))
            else:
                if cur: bands.append(cur)
                cur = [(k, best, s, e)]
        if cur: bands.append(cur)
        return bands

    for axis, name in (('h', '横向'), ('v', '纵向')):
        bands = scan_bands(axis)
        if not bands:
            continue
        print(f"{name}长条候选(连续内容 ≥250px):")
        for b in bands[:6]:
            ks = [r[0] for r in b]
            s = min(r[2] for r in b); e = max(r[3] for r in b)
            k0, k1 = min(ks), max(ks)
            if axis == 'h':
                print(f"  y={k0}..{k1}  x={s}..{e}  -> top={k0/H*100:.1f}% height={(k1-k0+1)/H*100:.1f}% "
                      f"left={s/W*100:.1f}% width={(e-s)/W*100:.1f}%")
            else:
                print(f"  x={k0}..{k1}  y={s}..{e}  -> left={k0/W*100:.1f}% width={(k1-k0+1)/W*100:.1f}% "
                      f"top={s/H*100:.1f}% height={(e-s)/H*100:.1f}%")


def main():
    args = sys.argv[1:]
    if not args:
        print(__doc__)
        sys.exit(0)
    for p in args:
        try:
            analyze(p)
        except Exception as e:
            print(f"[{p}] 分析失败: {e}")


if __name__ == "__main__":
    main()
