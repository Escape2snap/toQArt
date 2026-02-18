#!/usr/bin/env python3
# -*- coding: utf-8 -*-

from PIL import Image
import sys
import os
import glob

# 目标最小尺寸（像素）——最终宽度或高度将至少达到该值
TARGET_SIZE = 2000

def resize_images():
    """
    遍历 ./toqart 目录中的所有 PNG 文件
    将图像按整数倍放大，使得输出宽度或高度至少大于 `TARGET_SIZE`（默认 2000px），
    使用最近邻采样（保留像素风格）。
    """
    
    # 输入和输出目录
    input_dir = "./toqart"
    
    # 检查目录是否存在
    if not os.path.exists(input_dir):
        print(f"[ERROR] Directory {input_dir} does not exist", file=sys.stderr)
        sys.exit(1)
    
    # 查找所有 PNG 文件
    png_files = glob.glob(os.path.join(input_dir, "*.png"))
    
    if not png_files:
        print(f"[WARN] No PNG files found in {input_dir}")
        return

    print(f"[INFO] Found {len(png_files)} PNG files in {input_dir}")
    
    # 处理每个 PNG 文件
    for png_file in png_files:
        try:
            # 打开图像
            img = Image.open(png_file)
            original_width, original_height = img.size 
            
            # 计算缩放倍数，使用整数放大倍数，确保至少达到 TARGET_SIZE
            max_dim = max(original_width, original_height)
            scale = max(1, (TARGET_SIZE + max_dim - 1) // max_dim)

            new_width = original_width * scale
            new_height = original_height * scale

            # 使用最近邻采样（NEAREST）进行放大，保持像素风格
            resized_img = img.resize((new_width, new_height), Image.Resampling.NEAREST)
            
            # 覆盖原文件
            resized_img.save(png_file)
            
            print(f"[OK] Processed: {os.path.basename(png_file)}")
            print(f"[INFO] Original: {original_width}x{original_height} -> New: {new_width}x{new_height} (scale={scale}x)")
            
        except Exception as e:
            print(f"[ERROR] Failed to process file {png_file}: {str(e)}", file=sys.stderr)
    
    print("[OK] Done.")

if __name__ == "__main__":
    resize_images()
