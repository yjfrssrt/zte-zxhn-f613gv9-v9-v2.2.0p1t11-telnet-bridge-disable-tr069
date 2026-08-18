# 我们解决了2025年末新版难题：中兴 ZXHN F613GV9 V9.0 / V2.2.0P1T11 开Telnet、改桥接、真正关闭TR-069完全新手版

> 本项目在2026年实测解决：`ZXHN F613GV9 / V9.0 / V2.2.0P1T11`。
>
> 本文的 VLAN `4031` 来自河南移动这条实测线路。其他地区必须使用自己备份中的 VLAN。
>
> 只操作自己的设备。每一步看到“应看到”再继续；不一致就停。

需要原理、回滚、失败分析和可选优化时，看[标准文档](README.md)。

## 这份教程解决了什么

旧教程通常是：拔纤、重置、用通用超密登录、直接运行旧工具开 Telnet。老固件可以先参考[传统移动光猫桥接思路](https://jackcobra11.github.io/2022/01/CMCCmodem/)或开源项目 [zteOnu](https://github.com/Septrum101/zteOnu)。

但社区在2025年末和2026年的新固件上陆续遇到：

- FactoryMode 开始校验真实来源 MAC；
- 工具明明打印临时账号，23端口却不开；
- HTTPS 自签名证书让旧工具直接报错；
- 返回的加密数据长度异常，旧工具无法解密；
- 只在网页删除 TR-069，内部客户端仍继续运行。

社区更准确的说法是“2025年9月后的新方案”，不是所有设备都在12月同一天更新。[2026年新版工具讨论](https://www.right.com.cn/forum/thread-8476080-1-1.html)也提到了真实MAC检查和不完整响应问题。

**本项目的关键贡献**：我们在这台F613GV9上抓到了44字节异常响应，确认前32字节才是完整AES密文，后12字节是固件追加尾部；随后制作了只监听本机的HTTPS兼容代理，终于让旧客户端完成FactoryMode并实际登录Telnet。也就是说，这份是“旧简单教程失效以后”的困难版补丁路线，不是把旧教程重新抄一遍。

## 先认识五个词

| 名称 | 新手理解 |
| --- | --- |
| 光猫/ONT | 插光纤的中兴设备 |
| WAN | 光猫或路由器用于连接运营商的上网连接 |
| Bridge/桥接 | 光猫只转发数据，由自己的路由器负责拨号 |
| PPPoE | 路由器使用宽带账号密码拨号 |
| TR-069 | 运营商远程管理、改配置和改密码的通道 |

## 开始前准备

- Windows 电脑、纸和笔、正常网线、牙签/卡针；
- U盘可选：能导出完整配置当然最好，没有U盘也可以手抄关键参数；
- 宽带 PPPoE 账号密码；
- 光猫普通后台和超管凭据；
- 本项目提供的Windows新手工具包。

工具包内会预编译兼容代理，因此新手不需要自己安装Go。包内文件会逐项标明来源：PuTTY是MIT开源第三方软件；兼容代理是本项目产物；`go_build_zte_go.exe` 是未签名闭源社区附件，不是开源软件，也不是本项目编写。

## 第0步：先备份，绝对不能跳过

先登录 `http://192.168.1.1/`。拿纸笔手抄、手机离线拍照或保存到电脑都可以，U盘不是必须品。

### 三个最重要的“救命凭证”

请把下面三项抄两遍，逐个数字核对：

```text
① LOID
② PON Password（PON注册密码）
③ GPON SN（光猫序列号）
```

这三项决定光猫能否重新注册到运营商网络。抄错一个字符，后面就可能卡在PON注册。

然后继续记录：

```text
型号、硬件版本、软件版本
Internet WAN、VLAN、端口绑定
TR-069 WAN、VLAN
PPPoE 账号、密码
```

如果手头有U盘，再用光猫后台导出完整配置；没有U盘也不要慌，关键是先把上面内容抄完整。

### PPPoE账号密码是什么

它就是下级路由器“宽带拨号”时使用的宽带账号和密码，不是Wi-Fi密码，也不是光猫后台密码。

- 优先查看装机单、运营商短信或原路由器拨号设置；
- **本次河南移动线路实测**：使用办理宽带时绑定的手机号，编辑短信 `CZKDMM` 发送到 `10086`，系统会回复一个随机宽带密码；
- 其他省份的中国移动短信指令可能不同，可以先编辑 `10086` 发送到 `10086` 获取本省短信营业厅菜单，或直接拨打 `10086` 询问；
- 电信、联通等其他运营商请询问对应客服，不要照抄移动指令；
- 重置宽带密码后，原来正在拨号的光猫/路由器会掉线，稍后要使用新密码重新拨号。

中国移动官方的[短信营业厅说明](https://www.10086.cn/support/service/channel/sms/index.html)确认可以发送 `10086` 到 `10086` 获取菜单；[北京移动宽带自助排障](https://service.bj.10086.cn/bjyd/web/service/mobile/kdzz/index.html)使用的是带新密码参数的另一种格式，也说明短信指令需要参考当地规则。

**应看到：** 纸上至少有 LOID、PON Password、GPON SN、Internet VLAN和PPPoE凭据。三个救命凭证缺任何一个，都不要重置。

## 第1步：拔光纤，电脑直连

把光猫从原来的位置取下来，连同它自己的电源适配器一起拿到电脑旁边。现在只做下面几件事：

1. 拔掉光纤。光纤通常是插在 `PON` 口的绿色接头；手拿硬塑料接头拔，不要拽细细的光纤线。
2. 给光纤接头套上防尘帽或放在干净位置。不要用手摸前端的小圆面，也不要对着眼睛看。
3. 拔掉光猫和原路由器之间的网线。
4. 找一根正常网线，一头插电脑有线网口，另一头插光猫 `LAN1`。
5. 把光猫电源适配器插到电脑附近的插座，再给光猫供电。
6. 此时光猫不插光纤是故意的；LOS红灯亮或闪通常属于正常现象。

```text
墙上电源 ── 光猫电源适配器 ── 光猫
                                  │
电脑有线网口 ────── 网线 ───── LAN1

PON口：空着，不插光纤
其他口：先不插任何东西
```

## 第2步：设置电脑管理 IP

### 2.1 打开网卡设置

1. 同时按键盘 `Win + R`。
2. 在“运行”框输入：

```text
ncpa.cpl
```

3. 按回车，会打开“网络连接”。
4. 找到正在连接光猫的“以太网”。
5. 右键“以太网”→“属性”。
6. 双击“Internet协议版本4（TCP/IPv4）”。
7. 选择“使用下面的IP地址”。

填写：

```text
IP：192.168.1.2
掩码：255.255.255.0
网关：留空
DNS：留空
```

一路点击“确定”保存。

### 2.2 打开终端测试

1. 再按一次 `Win + R`。
2. 输入：

```text
powershell
```

3. 按回车。出现蓝色或黑色命令窗口就是PowerShell，也就是本文所说的“终端”。
4. 在光标后输入下面一行，再按回车：

```powershell
ping 192.168.1.1
```

5. 最后打开浏览器，在地址栏输入 `http://192.168.1.1/`。

**应看到：** PowerShell出现“来自 192.168.1.1 的回复”，浏览器也能打开光猫页面。若出现“请求超时”，检查LAN1、网线和刚才填写的IP，不能继续。

## 第3步：拔纤状态下重置

1. 再确认第0步已经备份，光纤仍然拔着。
2. 在光猫背面或侧面找到标着 `RESET` 的小圆孔。它不是普通按钮，手指按不到。
3. 拿牙签、卡针或回形针伸进小孔，轻轻顶住里面的小按钮；按到时能感觉到一点回弹。
4. 光猫通电状态下一直按住，大约10秒。看到指示灯整体熄灭、闪烁或设备开始重启后再松手。
5. 松开后什么也别按，耐心等待3～5分钟。
6. 再打开 `http://192.168.1.1/`。

**别做成“疯狂戳刺挑战”**：找准里面的按钮后持续按住即可，不需要大力捅，也不要反复短按。

## 第4步：登录超管，恢复注册资料

### 4.1 尝试移动出厂超管

中国移动光猫恢复出厂、并且仍未插光纤时，常见超管是：

```text
用户名：CMCCAdmin
密码：aDm8H%MdA
```

注意大小写和 `%`。这组凭据在本次F613GV9恢复出厂后实测可用，但不是所有型号/地区都保证有效；插回光纤后也可能被运营商改成动态密码。电信、联通或其他型号请按“运营商＋完整型号＋固件版本＋超密”查询，或询问当地装维/客服，不要乱试一长串密码。

### 4.2 写回你自己的注册资料

进入PON、LOID或设备注册页面。下面的方括号不是让你原样输入，而是提醒你填写第0步抄在纸上的原值：

```text
LOID = 【填写第0步抄下来的原LOID】
PON Password = 【填写第0步抄下来的原PON注册密码】
GPON SN = 【填写第0步抄下来的原GPON SN】
```

保存后重新进入页面复核。

**应看到：** 三个值与重置前完全一致。不要使用其他地区网上抄来的值，也不要插光纤。

## 第5步：先创建桥接

这一步按钮和下拉框比较多，不要一口气乱点。按顺序来：

1. 进入“网络”→“宽带设置”或“WAN设置”。
2. 点击“新建WAN连接”。
3. IP协议选择 `IPv4/IPv6`。
4. 连接模式选择 `Bridge`，中文通常叫“桥接”。
5. 业务模式选择 `INTERNET`。
6. VLAN模式选择 `Tag`、`改写`或“启用VLAN”。
7. VLAN ID填写**第0步从原Internet WAN抄下来的数字**。本次线路是4031，但你的线路可能完全不同。
8. DHCP Server、DHCP服务使能之类的勾选全部取消。
9. 如果页面还显示NAT，关闭NAT；Bridge模式下有些固件会自动隐藏这个选项。
10. 端口绑定推荐只勾 `LAN1`，以后路由器就固定接LAN1，最不容易混乱。

最终表单应类似：

```text
IP协议：IPv4/IPv6
模式：Bridge
业务：INTERNET
VLAN模式：Tag/改写
VLAN ID：【填写第0步抄下来的原Internet VLAN；本例才是4031】
端口绑定：仅 LAN1
NAT：关闭
DHCP Server：关闭
```

端口绑定并非所有固件都强制：不指定端口时，有的版本会让所有LAN口都可承载桥接。新手建议明确只绑定LAN1；熟悉网络、确实想让多个LAN口都桥接时才留空或全选。

11. 从第一项重新检查一遍。
12. 最后点击“创建”或“保存”。

**应看到：** 列表出现名称含 `INTERNET_B` 的连接。以本次为例是 `2_INTERNET_B_VID_4031`。重新点开仍是Bridge、LAN1和你自己的VLAN。没有成功前不要删旧WAN。

## 第6步：删除 TR-069 WAN

1. 选中名称含 `TR069` 的 WAN。
2. 再确认没有选中刚创建的 `INTERNET_B`。
3. 点击删除并刷新。

如果删除按钮灰色或无反应，按 `F12` 打开 Console，执行：

```js
(() => {
  const f = document.getElementById('mainFrame');
  const w = f?.contentWindow, d = w?.document;
  const s = d?.getElementById('Frm_WANCName0');
  const b = d?.getElementById('Btn_Delete');
  const name = s?.options[s.selectedIndex]?.text ?? '';
  if (!/TR[-_]?069/i.test(name)) throw new Error(`停止：选中的不是TR-069：${name}`);
  if (!b || typeof w.pageDel !== 'function' || typeof w.addEvent !== 'function')
    throw new Error('页面结构不同，停止操作');
  b.disabled = false;
  w.addEvent(b, 'click', w.pageDel);
  if (typeof w.handleRemoveClickEvent === 'function')
    w.addEvent(b, 'click', w.handleRemoveClickEvent);
  console.log(`已恢复删除按钮：${name}`);
})();
```

无报错后回到页面手动点删除。不要在 Console 里直接调用删除函数。

**应看到：** TR-069 WAN 消失，`2_INTERNET_B_VID_4031` 仍在且只绑定 LAN1。

### 重点：这里“删掉了”，但还没有彻底关闭

这一步确实调用了光猫页面自身的标准删除函数，`TR069_R_VID_xxxx` 这条WAN也确实从数据库的WANC列表消失了，不是假按钮。

但是我们随后通过Telnet检查发现：内部 `MgtServer` 仍然保存着ACS地址，`Tr069Enable`、周期上报和长连接仍是开启状态。也就是说，网页上看不见TR-069 WAN，并不等于后台远程管理程序彻底停止；理论上仍存在再次下发配置的可能。

目前只在本次F613GV9/固件上完成了数据库复核，不能断言所有中兴系列都完全相同。所以先记住一句话：**第6步只是删除可见WAN，第12步才是真正关闭内部TR-069。**

## 第7步：准备 FactoryMode 工具

为了避免新手研究“Go装哪、PuTTY点哪个”，本项目整理了Windows x64开源组件包：

[下载 F613GV9_beginner_tools_windows_x64_open_components.zip](https://github.com/yjfrssrt/zte-zxhn-f613gv9-v9-v2.2.0p1t11-telnet-bridge-disable-tr069/releases/download/v0.1.0/F613GV9_beginner_tools_windows_x64_open_components.zip)

ZIP的SHA-256：

```text
730E570D46D7E088679E870219B6D3F60093518228B9389A2806B75AB1A98851
```

### 新手怎么解压

1. 下载ZIP到桌面。
2. 右键ZIP→“全部解压缩”。
3. 点击“提取”。
4. 打开解压后的 `F613GV9_beginner_tools_windows_x64_open_components` 文件夹。
5. 先双击或打开 `README_FIRST.md`。

包内已经有：

```text
00_SHOW_MAC.cmd                  显示有线MAC
01_START_PROXY.cmd               启动本项目兼容代理
02_GET_TEMP_TELNET.cmd           中文提示获取临时Telnet
03_OPEN_PUTTY.cmd                打开Telnet客户端
04_EXPERIMENTAL_OPEN_CLIENT.cmd  运行本项目实验性开源客户端
f613gv9_factory_telnet.exe       本项目实验性开源客户端
zte_https_compat_proxy.exe       本项目产物
putty.exe                        PuTTY官方MIT开源软件
PLACE_GO_BUILD_HERE.txt          提醒第三方工具应放在哪里
SHA256SUMS.txt                   每个文件的哈希
```

`go_build_zte_go.exe` 不在包内。请打开[恩山论坛原始讨论第15页](https://www.right.com.cn/forum/thread-8461781-15-1.html)，在页面中找到其他用户分享的下载链接，自行下载。不要从本教程的二次网盘取得。

下载后检查：

```text
文件名：go_build_zte_go.exe
大小：5809664 bytes
SHA256：7D8ABF5FDCCE9E57A6C0939C91952DC1C706DB81C15FA5F9B42A8B299C947339
```

确认后，把它放进解压文件夹，与 `02_GET_TEMP_TELNET.cmd` 放在同一层。

验证ZIP：先按 `Win + R`，输入 `powershell`，按回车；再输入下面命令，把路径换成你下载的ZIP实际位置：

```powershell
Get-FileHash 'C:\Users\你的Windows用户名\Desktop\F613GV9_beginner_tools_windows_x64_open_components.zip' -Algorithm SHA256
```

屏幕上的哈希应与上面的ZIP哈希一致。`你的Windows用户名` 是中文说明，不能原样照抄。

PuTTY来自[官方0.85 x64版本](https://www.chiark.greenend.org.uk/~sgtatham/putty/latest.html)，MIT许可证随包附带。兼容代理源码在本仓库。`go_build_zte_go.exe` 未签名、无公开源码和再分发许可证，因此本项目只记录原始讨论页和实测哈希，不重新分发。

## 第8步：两条路线任选一条

### 路线A：本项目开源客户端（实验性质，不保证成功）

本项目重新实现了F613GV9专用客户端，直接处理HTTPS、真实MAC、44→32字节AES兼容和Telnet实际登录，不需要论坛闭源工具，也不需要先开兼容代理。

但是验证等级必须说清楚：公开协议、真实44字节响应和离线测试都已通过；作者自己的光猫已经完成配置并正常使用，不想为了验证新路径再次拆下、拔纤和重置，所以这条路线尚未在真实设备上完成端到端调用。愿意帮助测试可以使用，但**不保证成功**。

操作：

1. 双击 `04_EXPERIMENTAL_OPEN_CLIENT.cmd`。
2. 阅读黄色提示；确认是自己的F613GV9、已经拔纤并用网线直连后，输入大写 `YES`。
3. 输入真实有线MAC、超管用户名和超管密码。
4. 等待程序执行，不要重复点击。

程序只有实际登录Telnet shell成功后，才会打印临时用户名和密码。成功就抄下来并进入第9步；失败可改走路线B。

### 路线B：论坛工具＋本项目兼容代理（本次实机成功路线）

这是作者自己的F613GV9真正成功过的路径。传统工具会被自签名HTTPS和44字节异常响应卡住；本项目代理只把前32字节完整AES块交给旧客户端，从而完成FactoryMode。

1. 按第7步从论坛原页面自行取得并核对 `go_build_zte_go.exe`，放入解压目录。
2. 双击 `00_SHOW_MAC.cmd`，记下状态为 `Up` 的真实有线MAC。
3. 双击 `01_START_PROXY.cmd`。出现黑色窗口后不要关闭，这是“窗口A”。
4. 应看到 `listening on http://127.0.0.1:18080`。
5. 双击 `02_GET_TEMP_TELNET.cmd`。
6. 按中文提示输入有线MAC、超管用户名和超管密码。

成功时窗口会显示工具完成且23端口开放，并打印临时用户名和密码。把实际字符抄下来；“临时用户名”四个汉字不是用户名。

如果Windows安全软件阻止本项目程序，不要直接关闭杀毒软件。先核对 `SHA256SUMS.txt`；仍不放心就按标准文档从源码编译。

## 第9步：用PuTTY实际登录临时Telnet

无论选择路线A还是B，都必须做这一项：

1. 双击 `03_OPEN_PUTTY.cmd`。
2. PuTTY自动用Telnet连接 `192.168.1.1:23`。
3. 出现用户名提示时，输入所选路线刚刚打印的临时用户名，按回车。
4. 出现密码提示时，输入临时密码，按回车；输入时不显示星号是正常的。

连接参数应是：

```text
Host：192.168.1.1
Port：23
类型：Telnet
用户名：【填写路线A或B刚刚打印的临时用户名，不要原样输入本行】
密码：【填写路线A或B刚刚打印的临时密码，不要原样输入本行】
```

登录后执行：

```sh
uname -a
sendcmd 1 DB p TelnetCfg
```

**应看到：** PuTTY进入以 `#` 结尾的shell，`uname -a`和读取TelnetCfg都有输出。只打印账号但不能实际登录属于失败。路线B成功后可以回到窗口A按 `Ctrl + C` 关闭兼容代理。

## 第10步：生成永久强密码

按 `Win + R`，输入 `powershell`，按回车。复制下面三行到PowerShell，再按回车：

```powershell
$a='abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
$password=-join(1..32|ForEach-Object{$a[[Security.Cryptography.RandomNumberGenerator]::GetInt32($a.Length)]})
$password
```

最后一行会输出一串32位随机字母和数字，这就是你的永久Telnet密码。

**现在立刻拿纸抄两遍，或者保存到可信的密码管理器。** 不要只放在临时窗口里，也不要使用论坛示例密码。

忘记它不会让宽带立即断网，但以后将无法登录永久Telnet；届时通常只能重新执行第8～11步，再次获取临时Telnet并设置新密码。所以这张纸不是废纸，是未来少折腾一小时的“赎回券”。

## 第11步：固化 Telnet

继续使用第9步已经登录的PuTTY窗口。

### 11.1 先执行不含密码的设置

下面这些可以原样逐行执行：

```sh
sendcmd 1 DB p TelnetCfg
sendcmd 1 DB set TelnetCfg 0 Lan_Enable 1
sendcmd 1 DB set TelnetCfg 0 Wan_Enable 0
sendcmd 1 DB set TelnetCfg 0 InitSecLvl 3
sendcmd 1 DB set TelnetCfg 0 Max_Con_Num 5
sendcmd 1 DB set TelnetCfg 0 TS_UName root
sendcmd 1 DB set TelnetCfg 0 TSLan_UName root
```

每输入一行都按回车，看一眼有没有 `Access Denied`、`not exist` 或 `failed`。有错误就停。

### 11.2 再设置你自己的强密码

下面两行**绝对不能原样执行**。把每行最后的整段中文方括号删除，换成第10步纸上记录的32位密码：

```sh
sendcmd 1 DB set TelnetCfg 0 TS_UPwd 【这里替换成第10步记下的32位密码】
sendcmd 1 DB set TelnetCfg 0 TSLan_UPwd 【这里替换成同一个32位密码】
```

正确的一行末尾应该只有那32位英文字母和数字，不能留下 `【】` 或任何中文。

### 11.3 设置有效时间并保存

下面继续原样执行：

```sh
sendcmd 1 DB set TelnetCfg 0 ExitTime 999999
sendcmd 1 DB save
sendcmd 1 DB p TelnetCfg
```

本固件没有 `CloseServerTime` 和 `Lan_EnableAfterOlt`，不要强行添加。

**应看到：** 没有 `Access Denied`/`not exist`，并且 `Lan_Enable=1`、`Wan_Enable=0`。

## 第12步：真正关闭内部 TR-069

这一步就是修复第6步“网页删了、内部还开着”的问题。经历到这里后一般不会难：仍在同一个PuTTY窗口，一行一行复制、每行按回车即可。

先确认 WANC 只剩自己的桥接：

```sh
sendcmd 1 DB p MgtServer
sendcmd 1 DB p WANC
sendcmd 1 DB p PortBinding
```

确认无误后执行：

```sh
sendcmd 1 DB set MgtServer 0 URL http://127.0.0.1
sendcmd 1 DB set MgtServer 0 Tr069Enable 0
sendcmd 1 DB set MgtServer 0 PeriodicInformEnable 0
sendcmd 1 DB set MgtServer 0 PeriodicRandomEnable 0
sendcmd 1 DB set MgtServer 0 TcpLongConnectionEnable 0
sendcmd 1 DB set WANC 0 IsNAT 0
sendcmd 1 DB set UserIF 0 Timeout 120
sendcmd 1 DB save
sendcmd 1 DB p MgtServer
sendcmd 1 DB p WANC
sendcmd 1 DB p UserIF
```

**应看到：** TR-069 四个开关为0，`IsNAT=0`，`Timeout=120`。

这里没有需要替换的密码或中文占位符，可以原样执行。但前提是查询 `WANC` 时确实只剩你在第5步创建的Internet Bridge；若还有其他语音/IPTV/Internet连接，先停下查标准文档，不要盲目修改第0行。

新手版到这里即可，不必关闭全部防火墙/ALG。可选优化见[标准文档第14节](README.md#14-可选关闭桥接光猫的-upnp防火墙与-alg)。

## 第13步：重启并验证永久 Telnet

```sh
sendcmd 1 DB save
reboot
```

等3～5分钟，用 PuTTY 重新登录：

```text
192.168.1.1:23
用户：root
密码：【输入第10步纸上记下的32位密码，不要输入本句中文】
```

**应看到：** 重启后仍能登录。失败就不要插光纤，重新取得临时 Telnet 检查第11步。

## 第14步：插回光纤

插回前确认：PON三项正确、Bridge VLAN正确、仅绑定LAN1、TR-069 WAN已删、永久Telnet可登录、`Wan_Enable=0`。

确认后：

1. 插回光纤；
2. 等待 PON 灯稳定；
3. 后台确认 PON 状态达到 `O5`。

**应看到：** PON 为 O5。不到 O5 就检查 LOID、PON Password、GPON SN，不要继续。

## 第15步：连接路由器并拨号

1. 光猫 LAN1 接路由器 WAN。
2. 路由器选择“宽带拨号/PPPoE”。
3. 填写自己的 PPPoE 账号密码。
4. 路由器通常不再填 VLAN，因为光猫 Bridge 已负责 Tag。
5. 保存并连接。

**应看到：** PPPoE 已连接、IPv4/IPv6 可用、路由器 WAN 协商 `1000Mbps`。

- 认证失败：检查账号密码；
- 服务器无响应：检查 Bridge VLAN 和 LAN1；
- 只有约94Mbps：检查是否只协商100Mbps，更换网线/水晶头/端口。

## 第16步：恢复电脑自动获取 IP

回到第2步的 IPv4 页面，选择：

```text
自动获得 IP 地址
自动获得 DNS 服务器地址
```

## 最终勾选

- [ ] PON 为 O5；
- [ ] 路由器 PPPoE 已连接；
- [ ] WAN 协商1000Mbps；
- [ ] Bridge VLAN和LAN1正确；
- [ ] TR-069 WAN没有重新出现；
- [ ] `MgtServer` 开关均为0；
- [ ] 插纤并重启后，永久Telnet仍能登录；
- [ ] Telnet只允许LAN；

任何一步异常，停止并查[标准文档的失败路线与回滚](README.md#17-失败路线与排查)。

## 免责声明、非商业声明、开源许可与联系方式

### 免责声明

本文仅用于帮助设备所有者了解相关技术，并提供一种经过记录的研究与操作方法。刷写、重置、修改WAN/PON、关闭TR-069、开启Telnet或使用任何工具都可能造成断网、配置丢失、语音/IPTV失效、设备无法注册、失去运营商维护支持，甚至设备损坏。

读者必须确认自己对目标设备拥有合法管理权限，并自行判断型号、固件、地区和运营商配置是否适用。使用本文、脚本或代码所造成的一切直接或间接后果，由使用者自行承担；教程作者和项目贡献者不承担责任。实验性开源客户端尤其不保证能在未经实机验证的设备上成功。

### 非商业与无赞助声明

本文和本项目不涉及收费服务、商业利益、推广返佣或远程代开业务，没有接受设备厂商、运营商、论坛、网盘或第三方工具作者的赞助。文中链接仅用于注明资料来源和方便读者核对。

### 开源与署名

本项目自产代码全部公开，采用仓库中的MIT许可证。任何人都可以下载、学习、运行、修改和再分发，但必须保留原版权声明和许可证文本，不应把原项目代码或教程直接冒充为自己的原创成果。

PuTTY等第三方软件遵循各自许可证；未确认再分发许可的第三方程序不由本项目提供。

### 联系方式

联系邮箱：**a1470yi@163.com**

该邮箱是作者的子邮箱。遇到教程相关问题可以通过邮件提问，通常都会回复，但查看和回复可能比较慢，不承诺具体回复时限。作者没有在任何论坛注册账号；如需联系，请以该邮箱为准，不要相信论坛中的同名账号、代开服务或私聊收费人员。

具有代表性的问题和回复，可能在删除邮箱、姓名、账号、设备序列号、密码等个人或敏感信息后，经过适当的措辞、错别字和排版整理，追加到本教程末尾作为FAQ，技术含义不会故意改变。如果不希望自己的问题被匿名整理进教程，请在邮件中明确注明。

请勿通过邮件发送宽带密码、LOID、PON Password、GPON SN、完整配置文件或其他敏感凭据。
