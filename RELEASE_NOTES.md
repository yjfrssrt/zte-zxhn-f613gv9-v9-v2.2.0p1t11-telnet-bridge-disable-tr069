# v0.1.0

首次公开发布，适用实测设备：

```text
ZTE ZXHN F613GV9
Hardware V9.0
Software V2.2.0P1T11
```

包含：

- 标准技术实录与完全新手教程；
- HTTPS/self-signed certificate兼容代理；
- 44字节响应中32字节AES密文＋12字节尾部的严格兼容处理；
- 实验性开源F613GV9 FactoryMode/Telnet客户端；
- PuTTY和中文脚本组成的Windows x64开源组件包；
- VLAN 4031桥接、永久LAN Telnet和真正关闭TR-069的复核命令；
- 回滚、覆盖限制、免责声明和联系方式。

实验性开源客户端已经通过公开协议、真实44字节响应回归和离线单元测试，但尚未在下一次拔纤维护窗口完成实机端到端验证，不保证成功。

本Release不包含未签名、许可证未知的第三方 `go_build_zte_go.exe`。教程仅链接其原始社区讨论页并记录实测哈希。

Windows组件包：

```text
F613GV9_beginner_tools_windows_x64_open_components.zip
SHA256: 730E570D46D7E088679E870219B6D3F60093518228B9389A2806B75AB1A98851
```
