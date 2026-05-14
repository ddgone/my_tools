import sys
import time

def main():
    print("====================================")
    print("  欢迎使用 Python 演示脚本！")
    print("====================================")
    print(f"收到的命令行参数: {sys.argv[1:]}")
    
    print("\n正在执行一些模拟任务...")
    for i in range(1, 6):
        print(f"进度: {i * 20}%")
        time.sleep(0.5)
        
    print("\n任务完成！")

if __name__ == "__main__":
    main()
