import os
import subprocess
import ctypes
from ctypes import wintypes
import shutil
import signal
import sys
import tempfile
import time
from pathlib import Path

import typer
from rich.console import Console
from rich.panel import Panel
from rich.progress import Progress, SpinnerColumn, TextColumn
from rich.table import Table

if sys.platform == "win32":
    import winreg

app = typer.Typer(help="🚀 AI 聊天命令注册工具 - 使用 GPT-4.1 进行对话")
console = Console()


class CommandRegistrar:
    def __init__(self):
        self.temp_dir = tempfile.mkdtemp(prefix="python_cmd_")
        self.cmd_file = None
        self.cleaned_up = False
        self.profile_hook_file = None  # 临时 profile 脚本路径

    def register_chat_command(self):
        """注册chat命令"""
        start_time = time.time()

        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            console=console,
        ) as progress:
            task = progress.add_task("[cyan]正在注册 chat 命令...", total=None)

            if sys.platform == "win32":
                # 创建 Python 脚本文件
                chat_py_file = Path(self.temp_dir) / "chat_handler.py"
                with open(chat_py_file, "w", encoding="utf-8") as f:
                    f.write("""import sys
from openai import OpenAI
from rich.console import Console
from rich.markdown import Markdown
from rich.live import Live

# 初始化
console = Console()
client = OpenAI(api_key="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")

# 获取用户输入的消息
message = " ".join(sys.argv[1:])

if not message:
    console.print("[yellow]请输入消息内容，例如: chat 你好[/yellow]")
    sys.exit(1)

# 流式调用 OpenAI API
stream = client.chat.completions.create(
    model="gpt-4.1",
    messages=[{"role": "user", "content": message}],
    stream=True
)

# 累积响应内容并实时渲染 Markdown
full_response = ""
with Live(console=console, refresh_per_second=10) as live:
    for chunk in stream:
        if chunk.choices[0].delta.content is not None:
            full_response += chunk.choices[0].delta.content
            # 实时更新 Markdown 渲染
            live.update(Markdown(full_response))

console.print()  # 最后换行
""")

                # 创建批处理文件，使用当前 Python 解释器的完整路径
                self.cmd_file = Path(self.temp_dir) / "chat.bat"
                python_exe = sys.executable  # 获取当前 Python 解释器路径
                with open(self.cmd_file, "w", encoding="utf-8") as f:
                    f.write("@echo off\n")
                    f.write(f'"{python_exe}" "{chat_py_file}" %*\n')

                # 使用 winreg 直接操作注册表
                try:
                    key = winreg.OpenKey(
                        winreg.HKEY_CURRENT_USER,
                        r"Environment",
                        0,
                        winreg.KEY_READ | winreg.KEY_WRITE,
                    )
                    current_path, _ = winreg.QueryValueEx(key, "Path")
                    # 在PATH前面添加临时目录
                    new_path = f"{self.temp_dir};{current_path}"
                    winreg.SetValueEx(key, "Path", 0, winreg.REG_EXPAND_SZ, new_path)
                    winreg.CloseKey(key)
                except Exception:
                    pass

                # 让当前 Python 进程与其子进程立刻生效
                try:
                    os.environ["PATH"] = f"{self.temp_dir};" + os.environ.get("PATH", "")
                except Exception:
                    pass

                # 通知 Windows 更新环境变量（让 Explorer 启动的新进程也能看到）
                try:
                    _broadcast_env_change()
                except Exception:
                    pass

                # 为任意新开的 PowerShell 会话注入临时 hook（不依赖 PATH 顺序、无需后端配合）
                try:
                    self._install_powershell_profile_hook()
                except Exception as e:
                    console.print(f"[yellow]⚠ PowerShell profile 注入失败：[/yellow] {e}")

            progress.update(task, completed=True)

        elapsed_time = time.time() - start_time

        # 创建信息表格
        table = Table(show_header=False, box=None, padding=(0, 2))
        table.add_row("✨ 命令名称", "[bold cyan]chat[/bold cyan]")
        table.add_row("🤖 AI 模型", "[bold magenta]gpt-4.1[/bold magenta]")
        table.add_row("📍 命令位置", f"[dim]{self.cmd_file}[/dim]")
        table.add_row("📂 临时目录", f"[dim]{self.temp_dir}[/dim]")
        table.add_row("⏱️  注册耗时", f"[bold yellow]{elapsed_time:.3f}秒[/bold yellow]")

        console.print(
            Panel(
                table,
                title="[bold green]✅ Chat 命令注册成功[/bold green]",
                border_style="green",
            )
        )

        console.print("\n[yellow]⚠️  使用说明:[/yellow]")
        console.print("  [cyan]•[/cyan] 请打开 [bold]新的终端窗口[/bold]")
        console.print(
            "  [cyan]•[/cyan] 输入 [bold green]chat 你的消息[/bold green] 开始对话"
        )
        console.print(
            "  [cyan]•[/cyan] 例如: [bold green]chat 你好，介绍一下自己[/bold green]"
        )
        console.print(
            "  [cyan]•[/cyan] 按 [bold red]Ctrl+C[/bold red] 退出并自动清理\n"
        )

    def cleanup(self):
        """清理临时文件和目录"""
        if self.cleaned_up:
            return
        self.cleaned_up = True

        try:
            start_time = time.time()  # 开始计时

            with Progress(
                SpinnerColumn(),
                TextColumn("[progress.description]{task.description}"),
                console=console,
            ) as progress:
                task = progress.add_task("[yellow]正在清理...", total=None)

                # 直接从PATH中删除我们添加的临时目录
                if self.temp_dir:
                    try:
                        key = winreg.OpenKey(
                            winreg.HKEY_CURRENT_USER,
                            r"Environment",
                            0,
                            winreg.KEY_READ | winreg.KEY_WRITE,
                        )
                        current_path, _ = winreg.QueryValueEx(key, "Path")

                        # 过滤掉临时目录（保留其他所有条目）
                        path_list = [
                            p.strip()
                            for p in current_path.split(";")
                            if p.strip() and p.strip() != self.temp_dir
                        ]

                        new_path = ";".join(path_list)
                        winreg.SetValueEx(
                            key, "Path", 0, winreg.REG_EXPAND_SZ, new_path
                        )
                        winreg.CloseKey(key)
                    except Exception:
                        pass

                # 移除 PowerShell 注入的 hook
                try:
                    self._remove_powershell_profile_hook()
                except Exception:
                    pass

                # 删除临时目录
                if self.temp_dir and os.path.exists(self.temp_dir):
                    shutil.rmtree(self.temp_dir)

                progress.update(task, completed=True)

            elapsed_time = time.time() - start_time  # 计算耗时

            console.print(
                Panel(
                    "[green]✓[/green] 已清除 'chat' 命令\n"
                    "[green]✓[/green] 已删除临时目录\n"
                    "[green]✓[/green] 已恢复PATH环境变量\n"
                    f"[yellow]⏱️[/yellow]  清理耗时: [bold yellow]{elapsed_time:.3f}秒[/bold yellow]",
                    title="[bold blue]🧹 清理完成[/bold blue]",
                    border_style="blue",
                )
            )
        except Exception as e:
            console.print(f"[bold red]❌ 清理时出错:[/bold red] {e}")

    # 在 PowerShell 用户 profile 中注入一次性 hook，使任意新开会话可用 chat
    def _install_powershell_profile_hook(self):
        if sys.platform != "win32":
            return
        if not self.cmd_file:
            return
        self.profile_hook_file = Path(self.temp_dir) / "chat_profile.ps1"
        # 通过批处理包装，避免 Python 路径/虚拟环境问题
        snippet = (
            "$ErrorActionPreference = 'SilentlyContinue'\n"
            f"$env:Path = '{self.temp_dir};' + $env:Path\n"
            f"function chat {{ & '{self.cmd_file}' $args }}\n"
        )
        with open(self.profile_hook_file, "w", encoding="utf-8") as f:
            f.write(snippet)

        marker_start = "# >>> chatnext-terminal hook >>>"
        marker_end = "# <<< chatnext-terminal hook <<<"
        dot_line = f". '{self.profile_hook_file}'"

        docs = _user_documents_dir()
        targets = [
            docs / "WindowsPowerShell" / "Profile.ps1",  # Windows PowerShell 5.1
            docs / "PowerShell" / "Profile.ps1",         # PowerShell 7+
        ]
        for path in targets:
            try:
                _ensure_parent_dir(path)
                content = ""
                if path.exists():
                    content = path.read_text(encoding="utf-8", errors="ignore")
                    if marker_start in content and marker_end in content:
                        continue  # 已注入
                block = f"\n{marker_start}\n{dot_line}\n{marker_end}\n"
                with open(path, "a", encoding="utf-8") as pf:
                    pf.write(block)
            except Exception:
                # 忽略 profile 写入失败，不影响主流程
                pass

    # 从 PowerShell 用户 profile 移除注入的 hook
    def _remove_powershell_profile_hook(self):
        if sys.platform != "win32":
            return
        marker_start = "# >>> chatnext-terminal hook >>>"
        marker_end = "# <<< chatnext-terminal hook <<<"
        docs = _user_documents_dir()
        targets = [
            docs / "WindowsPowerShell" / "Profile.ps1",
            docs / "PowerShell" / "Profile.ps1",
        ]
        for path in targets:
            try:
                if not path.exists():
                    continue
                content = path.read_text(encoding="utf-8", errors="ignore")
                start = content.find(marker_start)
                end = content.find(marker_end)
                if start != -1 and end != -1:
                    end += len(marker_end)
                    new_content = content[:start].rstrip() + "\n" + content[end:].lstrip()
                    path.write_text(new_content, encoding="utf-8")
            except Exception:
                pass


# 全局变量用于信号处理
registrar = None


def _broadcast_env_change():
    if sys.platform != "win32":
        return
    user32 = ctypes.WinDLL("user32", use_last_error=True)
    HWND_BROADCAST = 0xFFFF
    WM_SETTINGCHANGE = 0x001A
    SMTO_NORMAL = 0x0000
    # LPARAM 为指向字符串 "Environment"
    SendMessageTimeoutW = user32.SendMessageTimeoutW
    SendMessageTimeoutW.argtypes = [wintypes.HWND, wintypes.UINT, wintypes.WPARAM, wintypes.LPARAM,
                                    wintypes.UINT, wintypes.UINT, ctypes.POINTER(wintypes.DWORD)]
    SendMessageTimeoutW.restype = wintypes.LPARAM
    result = wintypes.DWORD()
    SendMessageTimeoutW(HWND_BROADCAST, WM_SETTINGCHANGE, 0,
                        ctypes.cast(ctypes.c_wchar_p("Environment"), wintypes.LPARAM),
                        SMTO_NORMAL, 5000, ctypes.byref(result))

def _user_documents_dir() -> Path:
    # 尽量稳妥地定位“文档”目录
    home = Path(os.path.expanduser("~"))
    for name in ("Documents", "My Documents"):
        p = home / name
        if p.exists():
            return p
    return home / "Documents"

def _ensure_parent_dir(path: Path):
    path.parent.mkdir(parents=True, exist_ok=True)


def signal_handler(signum, frame):
    """处理中断信号"""
    console.print("\n[yellow]⚡ 收到中断信号，正在退出...[/yellow]")
    if registrar:
        registrar.cleanup()
    console.print("[dim]👋 再见！[/dim]\n")
    sys.exit(0)


@app.command()
def run(
    launch_terminal: bool = typer.Option(False, "--launch-terminal", help="注册后启动终端程序，使 Web 终端立即可用"),
    terminal_path: str = typer.Option("", "--terminal", help="指定终端可执行文件路径，留空则自动检测"),
):
    """🚀 启动命令注册服务（保持运行直到 Ctrl+C）"""
    global registrar
    registrar = CommandRegistrar()

    # 注册信号处理器
    signal.signal(signal.SIGINT, signal_handler)
    if sys.platform == "win32":
        signal.signal(signal.SIGBREAK, signal_handler)

    console.print(
        Panel.fit(
            "[bold cyan]🎯 系统命令注册工具[/bold cyan]\n[dim]按 Ctrl+C 退出[/dim]",
            border_style="cyan",
        )
    )

    try:
        # 注册命令
        registrar.register_chat_command()

        # 可选：启动 Go 终端（继承当前进程 PATH，从而无需在 Go 里特殊处理）
        if launch_terminal:
            exe_path = terminal_path
            if not exe_path:
                base_dir = Path(__file__).resolve().parent
                candidate = base_dir / "desktop" / "go" / "terminal.exe"
                if candidate.exists():
                    exe_path = str(candidate)
            if not exe_path:
                console.print("[yellow]未找到终端可执行文件，请通过 --terminal 指定路径[/yellow]")
            else:
                try:
                    subprocess.Popen([exe_path], env=os.environ.copy(), cwd=os.path.dirname(exe_path))
                    console.print(f"[green]✓ 已启动终端:[/green] [dim]{exe_path}[/dim]")
                except Exception as e:
                    console.print(f"[red]无法启动终端:[/red] {e}")

        # 保持脚本运行
        console.print("[dim]⏳ 服务运行中... (按 Ctrl+C 退出)[/dim]\n")
        while True:
            time.sleep(1)

    except KeyboardInterrupt:
        console.print("\n[yellow]⚡ 收到中断信号...[/yellow]")
    finally:
        if registrar:
            registrar.cleanup()
        console.print("[dim]👋 脚本已退出[/dim]\n")


@app.command()
def info():
    """📋 显示工具信息"""
    console.print(
        Panel(
            "[bold cyan]AI 聊天命令注册工具 v2.0[/bold cyan]\n\n"
            "[yellow]功能:[/yellow]\n"
            "  • 注册临时系统命令 'chat'\n"
            "  • 使用 GPT-4.1 模型进行对话\n"
            "  • 支持流式响应\n"
            "  • 自动管理PATH环境变量\n"
            "  • 退出时自动清理\n\n"
            "[yellow]使用方法:[/yellow]\n"
            "  [cyan]python plugins.py run[/cyan]  - 启动服务\n"
            "  [cyan]python plugins.py info[/cyan] - 显示此信息\n"
            "  [cyan]chat 你的消息[/cyan]        - 与 AI 对话（需在新终端窗口）",
            title="[bold green]ℹ️  工具信息[/bold green]",
            border_style="green",
        )
    )


if __name__ == "__main__":
    # 如果没有提供命令参数，默认运行 run 命令
    if len(sys.argv) == 1:
        sys.argv.append("run")
    app()
