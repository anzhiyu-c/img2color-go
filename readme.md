# Img2color

本项目使用go作为基础，具有较高的性能

支持vercel与服务器部署

## vercel部署

1. 点击项目右上角fork叉子

2. 登录[vercel](https://vercel.com/)

3. 在[vercel](https://vercel.com/)导入项目

4. 国内访问需绑定自定义域名

## 服务器部署

需要go环境

1. 安装依赖
```bash
go mod tidy
```
2. 运行
```
go run /api/img2color.go
```
此处不赘述守护进程。

## 使用

例如：https://img2color-go.vercel.app/img2color?img=https://npm.elemecdn.com/anzhiyu-blog@1.1.6/img/post/banner/神里.webp

部署后只需要 域名/img2color 访问

必填参数img: url