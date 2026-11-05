export const repo = 'https://github.com/mygo-music/mygo-music'

export type Asset = { name: string; size: number; browser_download_url: string }
export type Release = { tag_name: string; published_at: string; assets: Asset[] }

declare const __RELEASE__: Release
// The latest release, as it was when the site was built (see vite.config.ts).
export const release = __RELEASE__
export const mygo = 'https://mygo.egoist.dev'

export const site = {
  name: 'MyGo 音乐',
  title: 'MyGo 音乐 — 音乐来源可由插件扩展的原生播放器',
  description:
    '用 Go 与 MyGo 写成的跨平台原生音乐播放器，音乐来源可由插件扩展。原生界面，没有 WebView；边下边播，无损与 Hi-Res，逐字歌词与「律动」，刘海播放器。',
}
