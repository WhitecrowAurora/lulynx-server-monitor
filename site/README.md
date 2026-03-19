# Lulynx Site

这个目录放的是项目附带的独立站点工程，基于 React + Vite。

它和 `cmd/center/web` 不是一回事：

- `cmd/center/web` 是真正内嵌进 `tanzhen-center` 的监控主页和管理面板
- `site/` 更像官网、展示页或者介绍页，单独开发、单独构建

## 本地开发

```bash
npm install
npm run dev
```

## 构建

```bash
npm run build
```

构建产物会输出到 `site/dist/`，这个目录不需要提交到仓库。
