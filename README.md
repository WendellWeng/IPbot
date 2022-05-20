### "IP查询机器人"设计方案

#### Github地址

+ https://github.com/WendellWeng/IPbot

#### 项目简介

+ 该项目简单实现了QQ频道机器人与用户之间的交互逻辑，主要参考了QQ官方的`SDK`-`botgo`的实现方法
+ 技术栈: Golang语言、Redis
+ 应用层协议: `websocket`、`HTTP`

#### 项目需求

+ 目前QQ频道刚刚兴起，频道机器人的种类不多，大致浏览了QQ频道中的一些机器人，没有发现有提供IP查询的功能，同时出于学习的目的，于是有了开发该项目的念头；
+ IP查询虽然在大众生活当中不起眼，但是有些场景确实非常便利，当程序猿切换代理时，打开QQ，通过IP查询机器人就可以知道自己的IP所属的区域，可以不通过浏览器就可以查询了。

#### 整体架构

<img src="https://tva1.sinaimg.cn/large/e6c9d24egy1h2etr01ejzj20r00esjsf.jpg" alt="fig01" style="zoom:67%;" />

+ 首先，用户将查询请求发送给QQ频道服务器；
+ QQ频道服务器通过`websocket`连接将用户的请求推送给机器人；
+ 机器人解析服务器推送过来的信息，提取其中的待查询的**IP信息**；
+ 根据IP，第一步先查询Redis数据库是否存在该IP。若Redis数据库中存在该IP，则直接返回IP结果；若不存在，则通过HTTP请求查询，并将查询的结果存储至Redis数据库；
+ 获取到查询结果后，机器人通过HTTP将查询结果回复到频道当中。

#### 通信交互

<img src="https://tva1.sinaimg.cn/large/e6c9d24egy1h2etr3dsb7j20jr0fvgmb.jpg" alt="fig02"  />

+ 机器人客户端向服务器发送`HTTPS`请求连接，其中`GET`请求携带用于验证的`Token`；
+ 服务器响应请求，在响应体中主要携带了建立`websocket`连接的`URL`和分片(相当于机器人的频道个数)；
+ 机器人客户端使用新的`URL`发起`websocket`连接请求，服务器会响应`websocket`连接的心跳信息以及操作码；
+ 此时机器人客户端必须发送一次鉴权信息给服务器才能让服务器提供服务，鉴权信息包括机器人的`Token`值、回调函数的`intent`值、分片信息、以及操作码；
+ 服务器验证后，并响应版本号、`session_id`、机器人`id`、机器人名称等；
+ 机器人客户端记录上一步骤的响应信息，开始监听`websocket`连接并提供查询服务；
+ 用户向服务器发起IP查询请求，此时服务器通过`websocket`连接将用户的查询信息推送至机器人客户端；
+ 机器人客户端接收服务器的推送数据后，主要解析并提取出其中的频道`id`(用于消息回复)以及用户的输入内容`content`；
+ 机器人客户端提取输入内容`content`的IP字符串，查询其结果，并将其结果封装到`POST`请求的`body`当中;
+ 机器人客户端根据频道`id`组装`URL`(相当于路由)，将查询结果通过`HTTP`发送至用户所在频道中。

#### 业务逻辑

![fig03](https://tva1.sinaimg.cn/large/e6c9d24egy1h2etr724s6j214b0f1t9v.jpg)

+ 机器人客户端解析服务器推送的查询内容，提取其中的IP字符串，向Redis数据库发起查询；
+ 数据库返回查询结果，机器人客户端判断缓存是否命中；
+ 若缓存命中，则直接封装查询结果并推送至用户所在的频道当中，服务结束；
+ 若缓存未命中，则机器人客户端需要发起`HTTP`请求，向互联网查询IP的详细信息，并接收查询结果。机器人客户端将查询结果缓存至Redis当中，以便之后可以快速获取结果，同时也将查询结果推送至用户所在频道当中，服务结束。

#### 代码结构

![fig04](https://tva1.sinaimg.cn/large/e6c9d24egy1h2etrhodv8j206h0aojrg.jpg)

+ robot.go: 主函数，注册回调函数，发起`HTTPS`请求，并建立`session`会话；
+ clien.go: 接收服务器推送的消息，建立监听线程，并根据操作码，选择对应的业务逻辑进行处理；
+ base.go: `Message`结构体、`PayLoad`结构体、`IPResult`结构体等数据结构都在该文件中声明，并且提供数据解析函数`ParseData`，指令解析函数`ParseCommand`；
+ token.go: 声明了`Token`结构，以及`token`生成函数(`appID + token`)；
+ config.yaml: 配置文件，存储了机器人以及一些`API`接口的`ID`和`Token`，

#### 效果展示

+ 输入正常`IP`时, 显示正常的查询结果

<img src="https://tva1.sinaimg.cn/large/e6c9d24egy1h2etrkwd6aj20ja0jqjsr.jpg" alt="Screen Shot 2022-05-20 at 1.28.20 PM" style="zoom:50%;" />

+ 输入多个指令时，目前只支持解析第一个指令，忽略掉第一个后面的指令

  <img src="https://tva1.sinaimg.cn/large/e6c9d24egy1h2etrogyizj20ja0ksabq.jpg" alt="Screen Shot 2022-05-20 at 1.28.48 PM" style="zoom:50%;" />

+ 输入空的`IP`字符串时，机器人会响应错误提示

  <img src="https://tva1.sinaimg.cn/large/e6c9d24egy1h2etrrg9tuj20ja0fygmu.jpg" alt="Screen Shot 2022-05-20 at 1.28.39 PM" style="zoom:50%;" />

+ 输入的`IP`不合法或者非法字符串(例如包含除数字和点以外其他字符等情况)时，机器人会报告`IP`不合法

  <img src="https://tva1.sinaimg.cn/large/e6c9d24egy1h2etrueyzoj20ja0eoq3y.jpg" alt="Screen Shot 2022-05-20 at 1.28.57 PM" style="zoom:50%;" />



<img src="https://tva1.sinaimg.cn/large/e6c9d24egy1h2etrx69cjj20ja0fyab6.jpg" alt="Screen Shot 2022-05-20 at 1.28.33 PM" style="zoom:50%;" />



+ 输入的指令不合法时，机器人会响应错误提示

  <img src="https://tva1.sinaimg.cn/large/e6c9d24egy1h2ets0mxgwj20ja0g475d.jpg" alt="Screen Shot 2022-05-20 at 1.31.45 PM" style="zoom:50%;" />

#### 不足与展望

##### 收获点

+ 借鉴官方`SDK`的实现并脱离官方的`SDK`，自己实现了QQ频道机器人的交互逻辑；
+ 通过学习官方`SDK`源码，夯实个人的`Golang`基础，对QQ频道底层的通信有了更深的了解；

##### 局限性

+ 目前该项目所实现的功能比较有限，仅支持处理机器人这一种消息格式，没有官方`SDK`的功能之多，但是该项目通信交互完整，其他功能可以待后期添加，功能堆砌相对简单；
+ 由于时间有限，该项目不支持创建多个`session`会话(分片功能)，即不支持为多频道的机器人提供服务；
+ 该项目暂时没有提供持久化服务，所有数据仅缓存在内存当中。

##### 展望

+ 为该项目添加更多有趣的功能，比如查看当天新闻，来一段搞笑段子，查询天气等等；
+ 为该项目添加分片功能，支持同时为多个频道进行服务；
+ 为该项目添加持久化，可以存储到MySQL当中，这样也能为用户提供查询历史的功能；
+ 该项目在处理用户请求时，采用的是"单线程+消息队列"的机制来实现的，未来可以考虑加入线程池来为大规模用户提供服务。