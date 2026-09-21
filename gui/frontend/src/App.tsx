import {
  ArrowDownToLine,
  ArrowUpFromLine,
  Check,
  CircleCheck,
  CircleX,
  Clock3,
  Database,
  FolderOpen,
  MessageSquare,
  RefreshCcw,
  Search,
  Settings,
  Square,
  Upload,
  X,
} from 'lucide-react';
import {useEffect, useMemo, useState} from 'react';

type TopicRef = {id: number; title: string};
type ChatRef = {id: number; username: string; title: string; type: string; topics: TopicRef[]; self: boolean};
type ChatPage = {items: ChatRef[]; next: number; skipped: number};
type DesktopSettings = {schemaVersion: number; namespace: string; proxy: string; threads: number; limit: number; downloadDirectory: string; coverMode: string; coverAt: string; downloadGroup: boolean; downloadSkipSame: boolean; downloadRewrite: boolean};
type TransferItem = {
  TaskID: string;
  FileName: string;
  SourcePath: string;
  CompletedBytes: number;
  TotalBytes: number;
  Status: string;
  Phase: string;
  Err: string;
  At: string;
};
type Snapshot = {
  Pending: number;
  Running: number;
  Succeeded: number;
  Failed: number;
  Canceled: number;
  Skipped: number;
  CompletedBytes: number;
  TotalBytes: number;
  Speed: number;
  CurrentFile: string;
  DiscoveryDone: boolean;
  Final: boolean;
  Unknown: number;
  Status: string;
  Errors: string[];
};

const emptySnapshot: Snapshot = {
  Pending: 0,
  Running: 0,
  Succeeded: 0,
  Failed: 0,
  Canceled: 0,
  Skipped: 0,
  CompletedBytes: 0,
  TotalBytes: 0,
  Speed: 0,
  CurrentFile: '',
  DiscoveryDone: false,
  Final: false,
  Unknown: 0,
  Status: '',
  Errors: [],
};

const savedMessages: ChatRef = {id: 0, username: '', title: 'Saved Messages', type: 'self', topics: [], self: true};
const defaultSettings: DesktopSettings = {schemaVersion: 1, namespace: 'default', proxy: '', threads: 4, limit: 2, downloadDirectory: 'downloads', coverMode: 'video-cover', coverAt: 'auto', downloadGroup: true, downloadSkipSame: true, downloadRewrite: false};
type WorkspacePage = 'upload' | 'download' | 'settings';
const nav = [
  [Upload, '上传', 'upload'],
  [ArrowDownToLine, '下载', 'download'],
  [MessageSquare, '会话', ''],
  [ArrowUpFromLine, '任务', ''],
  [Database, '备份', ''],
  [RefreshCcw, '恢复', ''],
  [Settings, '设置', 'settings'],
] as const;

export function App() {
  const [paths, setPaths] = useState<string[]>([]);
  const [activePage, setActivePage] = useState<WorkspacePage>('upload');
  const [downloadInput, setDownloadInput] = useState('');
  const [downloadFiles, setDownloadFiles] = useState<string[]>([]);
  const [downloadDirectory, setDownloadDirectory] = useState('downloads');
  const [downloadGroup, setDownloadGroup] = useState(true);
  const [downloadSkipSame, setDownloadSkipSame] = useState(true);
  const [downloadRewrite, setDownloadRewrite] = useState(false);
  const [settings, setSettings] = useState<DesktopSettings>(defaultSettings);
  const [settingsDraft, setSettingsDraft] = useState<DesktopSettings>(defaultSettings);
  const [settingsStatus, setSettingsStatus] = useState('');
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [namespace, setNamespace] = useState('default');
  const [running, setRunning] = useState(false);
  const [error, setError] = useState('');
  const [snap, setSnap] = useState<Snapshot>(emptySnapshot);
  const [items, setItems] = useState<TransferItem[]>([]);
  const [chatOpen, setChatOpen] = useState(false);
  const [chatItems, setChatItems] = useState<ChatRef[]>([]);
  const [chatNext, setChatNext] = useState(0);
  const [chatLoading, setChatLoading] = useState(false);
  const [chatError, setChatError] = useState('');
  const [chatQuery, setChatQuery] = useState('');
  const [selectedChat, setSelectedChat] = useState<ChatRef>(savedMessages);
  const [selectedTopic, setSelectedTopic] = useState<TopicRef | null>(null);
  const [draftChat, setDraftChat] = useState<ChatRef>(savedMessages);
  const [draftTopic, setDraftTopic] = useState<TopicRef | null>(null);

  useEffect(() => {
    Promise.allSettled([window.go?.main.App.Namespaces(), window.go?.main.App.GetSettings()]).then(([namespaceResult, settingsResult]) => {
      const values = namespaceResult.status === 'fulfilled' ? namespaceResult.value : [];
      setNamespaces(values);
      if (namespaceResult.status === 'rejected') setError(errorText(namespaceResult.reason));
      if (settingsResult.status === 'fulfilled') {
        const loaded = settingsResult.value;
        setSettings(loaded);
        setSettingsDraft(loaded);
        setDownloadDirectory(loaded.downloadDirectory);
        setDownloadGroup(loaded.downloadGroup);
        setDownloadSkipSame(loaded.downloadSkipSame);
        setDownloadRewrite(loaded.downloadRewrite);
        setNamespace(values.includes(loaded.namespace) ? loaded.namespace : values[0] || loaded.namespace);
      } else {
        setError(errorText(settingsResult.reason));
        if (values.length) setNamespace(values.includes('default') ? 'default' : values[0]);
      }
    });
    const offSnapshot = window.runtime?.EventsOn('transfer:snapshot', (value: Snapshot) => setSnap(value));
    const offItems = window.runtime?.EventsOn('transfer:items', (value: TransferItem[]) => setItems(value || []));
    const offDone = window.runtime?.EventsOn('transfer:done', (value: {error?: string}) => {
      setRunning(false);
      setError(value.error || '');
    });
    return () => {
      offSnapshot?.();
      offItems?.();
      offDone?.();
    };
  }, []);

  useEffect(() => {
    setSelectedChat(savedMessages);
    setSelectedTopic(null);
    setChatItems([]);
    setChatNext(0);
    setChatError('');
  }, [namespace]);

  const knownTotal = snap.DiscoveryDone && snap.Unknown === 0 && snap.TotalBytes > 0;
  const pct = knownTotal ? Math.min(100, (snap.CompletedBytes * 100) / snap.TotalBytes) : 0;
  const eta = knownTotal && snap.Speed > 0 && !snap.Final ? duration((snap.TotalBytes - snap.CompletedBytes) / snap.Speed) : '--';
  const targetLabel = selectedTopic ? `${selectedChat.title} · ${selectedTopic.title}` : selectedChat.title;
  const filteredChats = useMemo(() => {
    const query = chatQuery.trim().toLocaleLowerCase();
    if (!query) return chatItems;
    return chatItems.filter((entry) => `${entry.title} ${entry.username} ${entry.type}`.toLocaleLowerCase().includes(query));
  }, [chatItems, chatQuery]);
  const downloadURLs = useMemo(() => downloadInput.split(/\s+/).map((value) => value.trim()).filter(Boolean), [downloadInput]);
  const settingsDirty = JSON.stringify(settingsDraft) !== JSON.stringify(settings);

  function switchPage(page: WorkspacePage) {
    if (running || page === activePage) return;
    if (activePage === 'settings' && settingsDirty && !window.confirm('设置尚未应用，确定放弃修改吗？')) return;
    if (page === 'settings') {
      setSettingsDraft(settings);
      setSettingsStatus('');
    }
    setActivePage(page);
    setError('');
    setSnap(emptySnapshot);
    setItems([]);
  }

  async function pickFiles() {
    const value = await window.go.main.App.SelectUploadFiles();
    if (value?.length) setPaths(value);
  }

  async function pickDirectory() {
    const value = await window.go.main.App.SelectUploadDirectory();
    if (value) setPaths([value]);
  }

  async function pickDownloadDirectory() {
    const value = await window.go.main.App.SelectDownloadDirectory();
    if (value) setDownloadDirectory(value);
  }

  async function pickDownloadFiles() {
    const value = await window.go.main.App.SelectDownloadExportFiles();
    if (value?.length) setDownloadFiles(value);
  }

  async function pickSettingsDownloadDirectory() {
    const value = await window.go.main.App.SelectDownloadDirectory();
    if (value) setSettingsDraft((current) => ({...current, downloadDirectory: value}));
  }

  async function loadChats(cursor = 0, reset = false) {
    setChatLoading(true);
    setChatError('');
    try {
      const page: ChatPage = await window.go.main.App.ChatPage(namespace, cursor, 40);
      setChatItems((previous) => uniqueChats(reset ? page.items : [...previous, ...page.items]));
      setChatNext(page.next || 0);
    } catch (reason) {
      setChatError(errorText(reason));
    } finally {
      setChatLoading(false);
    }
  }

  async function openChatSelector() {
    setDraftChat(selectedChat);
    setDraftTopic(selectedTopic);
    setChatOpen(true);
    if (!chatItems.length) await loadChats(0, true);
  }

  function confirmChatSelector() {
    setSelectedChat(draftChat);
    setSelectedTopic(draftTopic);
    setChatOpen(false);
  }

  async function startUpload() {
    setError('');
    setSnap(emptySnapshot);
    setItems([]);
    try {
      await window.go.main.App.StartUpload({
        namespace,
        proxy: settings.proxy,
        paths,
        chat: selectedChat.self ? '' : String(selectedChat.id),
        topic: selectedTopic?.id || 0,
        coverMode: settings.coverMode,
        coverAt: settings.coverAt,
        threads: settings.threads,
        limit: settings.limit,
      });
      setRunning(true);
    } catch (reason) {
      setError(errorText(reason));
    }
  }

  async function startDownload() {
    setError('');
    setSnap(emptySnapshot);
    setItems([]);
    try {
      await window.go.main.App.StartDownload({
        namespace,
        proxy: settings.proxy,
        urls: downloadURLs,
        files: downloadFiles,
        directory: downloadDirectory,
        threads: settings.threads,
        limit: settings.limit,
        group: downloadGroup,
        skipSame: downloadSkipSame,
        rewrite: downloadRewrite,
      });
      setRunning(true);
    } catch (reason) {
      setError(errorText(reason));
    }
  }

  async function saveSettings() {
    setSettingsStatus('');
    try {
      const saved = await window.go.main.App.SaveSettings(settingsDraft);
      setSettings(saved);
      setSettingsDraft(saved);
      setDownloadDirectory(saved.downloadDirectory);
      setDownloadGroup(saved.downloadGroup);
      setDownloadSkipSame(saved.downloadSkipSame);
      setDownloadRewrite(saved.downloadRewrite);
      if (namespaces.includes(saved.namespace)) setNamespace(saved.namespace);
      setSettingsStatus('设置已保存并应用');
    } catch (reason) {
      setSettingsStatus(errorText(reason));
    }
  }

  return (
    <main className="shell">
      <aside className="sidebar">
        <section className="brand"><strong>TDL</strong><span>Telegram Media Transfer</span><em>传 输 工 作 台</em></section>
        <nav>{nav.map(([Icon, label, page]) => <button disabled={!page || running} title={!page ? '后续阶段实现' : ''} className={page === activePage ? 'active' : ''} key={label} onClick={() => page && switchPage(page)}><Icon size={21}/><span>{label}</span></button>)}</nav>
        <footer><div>桌面版 · 本地运行</div><b>让传输更简单</b><small>MEDIA ANYWHERE<br/>WITH TDL</small></footer>
      </aside>

      <section className="workspace">
        <header className="session">
          <span>工作区：<b>{activePage === 'upload' ? '上传' : activePage === 'download' ? '下载' : '设置'}</b></span><i/>
          <span>账号：<select disabled={running || chatLoading || activePage === 'settings'} value={namespace} onChange={(event) => setNamespace(event.target.value)}>{namespaces.map((value) => <option key={value}>{value}</option>)}</select></span><i/>
          <span>状态：<mark>● {running ? '传输中' : '就绪'}</mark></span>
          <time>{new Date().toLocaleString()}</time>
        </header>

        {activePage === 'settings' ? <article className="settings-page card">
          <header><div><small>DESKTOP PREFERENCES</small><h1>全局设置</h1><p>这些值会应用到之后启动的上传和下载任务。保存失败时，当前生效值不会改变。</p></div><Settings size={34}/></header>
          <div className="settings-grid">
            <label><span>默认账号<small>新任务默认使用的 Telegram 命名空间</small></span><select value={settingsDraft.namespace} onChange={(event) => setSettingsDraft({...settingsDraft, namespace: event.target.value})}>{namespaces.map((value) => <option key={value}>{value}</option>)}</select></label>
            <label><span>代理地址<small>留空时直接连接，例如 socks5://127.0.0.1:1080</small></span><input value={settingsDraft.proxy} onChange={(event) => setSettingsDraft({...settingsDraft, proxy: event.target.value})} placeholder="protocol://host:port"/></label>
            <label><span>单文件线程数<small>一个文件内部的并行传输线程，范围 1–64</small></span><input type="number" min="1" max="64" value={settingsDraft.threads} onChange={(event) => setSettingsDraft({...settingsDraft, threads: Number(event.target.value)})}/></label>
            <label><span>并行任务数<small>同时处理的文件数量，范围 1–32</small></span><input type="number" min="1" max="32" value={settingsDraft.limit} onChange={(event) => setSettingsDraft({...settingsDraft, limit: Number(event.target.value)})}/></label>
            <label><span>视频封面模式<small>video_cover 高清封面不会改变视频起播时间</small></span><select value={settingsDraft.coverMode} onChange={(event) => setSettingsDraft({...settingsDraft, coverMode: event.target.value})}><option value="video-cover">video_cover · 高清</option><option value="video-thumb">video_thumb · 快速</option><option value="none">不生成封面</option></select></label>
            <label><span>封面取帧时间<small>auto 自动选择，也可输入 2s、00:00:05</small></span><input value={settingsDraft.coverAt} onChange={(event) => setSettingsDraft({...settingsDraft, coverAt: event.target.value})} placeholder="auto"/></label>
            <label className="wide"><span>默认下载目录<small>下载页首次打开和应用设置后使用的目录</small></span><div className="path-setting"><input readOnly value={settingsDraft.downloadDirectory}/><button onClick={pickSettingsDownloadDirectory}><FolderOpen size={16}/>选择</button></div></label>
            <label className="wide"><span>下载默认行为<small>每次仍可在下载页单独修改</small></span><div className="setting-checks"><label><input type="checkbox" checked={settingsDraft.downloadGroup} onChange={(event) => setSettingsDraft({...settingsDraft, downloadGroup: event.target.checked})}/> 自动下载媒体组</label><label><input type="checkbox" checked={settingsDraft.downloadSkipSame} onChange={(event) => setSettingsDraft({...settingsDraft, downloadSkipSame: event.target.checked})}/> 跳过同名同大小</label><label><input type="checkbox" checked={settingsDraft.downloadRewrite} onChange={(event) => setSettingsDraft({...settingsDraft, downloadRewrite: event.target.checked})}/> 修正扩展名</label></div></label>
          </div>
          <footer><span className={settingsStatus === '设置已保存并应用' ? 'settings-ok' : 'settings-error'}>{settingsStatus || (settingsDirty ? '有尚未应用的修改' : '设置已与磁盘同步')}</span><div><button disabled={!settingsDirty} onClick={() => {setSettingsDraft(settings); setSettingsStatus('已放弃未应用的修改')}}>取消修改</button><button className="apply" disabled={!settingsDirty} onClick={saveSettings}>应用设置</button></div></footer>
        </article> : <>
        <div className="overview">
          <article className="media card">
            <label>{activePage === 'upload' ? 'UPLOAD TARGET' : 'DOWNLOAD SOURCE'}</label>
            {activePage === 'upload'
              ? <div className="cover"><MessageSquare size={32}/><span>{selectedChat.self ? 'SAVED' : selectedChat.type.toUpperCase()}</span><small>{targetLabel}</small></div>
              : <div className="cover download-cover"><ArrowDownToLine size={32}/><span>LINKS</span><small>{downloadURLs.length} 个链接 · {downloadFiles.length} 个导出文件</small></div>}
          </article>

          <article className="task card">
            <h2>{activePage === 'upload' ? <Upload/> : <ArrowDownToLine/>} 当前任务 <span>· {activePage === 'upload' ? '上传' : '下载'}</span></h2><hr/>
            {activePage === 'upload' ? <>
              <h1>{snap.CurrentFile || paths[0]?.split(/[\\/]/).pop() || '选择要上传的文件'}</h1>
              <p>{paths.length ? `已选择 ${paths.length} 项` : '支持文件和目录 · 视频默认生成高清封面'}</p>
              <div className="target-row"><span>发送到</span><button disabled={running} onClick={openChatSelector}><MessageSquare size={15}/>{targetLabel}</button></div>
              <div className="picker"><button disabled={running} onClick={pickFiles}>选择文件</button><button disabled={running} onClick={pickDirectory}>选择目录</button><button className="primary" disabled={!paths.length || running} onClick={startUpload}>开始上传</button></div>
            </> : <>
              <textarea className="download-input" disabled={running} value={downloadInput} onChange={(event) => setDownloadInput(event.target.value)} placeholder={'粘贴 Telegram 消息链接，每行一个\n例如：https://t.me/c/123456/789'}/>
              <div className="download-destination"><span>保存到</span><button disabled={running} onClick={pickDownloadDirectory}><FolderOpen size={15}/><b>{downloadDirectory}</b></button><button disabled={running} onClick={pickDownloadFiles}>导入 JSON</button></div>
              <div className="download-options"><label><input type="checkbox" checked={downloadGroup} onChange={(event) => setDownloadGroup(event.target.checked)}/> 自动下载媒体组</label><label><input type="checkbox" checked={downloadSkipSame} onChange={(event) => setDownloadSkipSame(event.target.checked)}/> 跳过同名同大小</label><label><input type="checkbox" checked={downloadRewrite} onChange={(event) => setDownloadRewrite(event.target.checked)}/> 修正扩展名</label><button className="primary" disabled={(!downloadURLs.length && !downloadFiles.length) || running} onClick={startDownload}>开始下载</button></div>
            </>}
            <div className={`progress ${knownTotal ? '' : 'indeterminate'}`}><span style={knownTotal ? {width: `${pct}%`} : undefined}/><b>{knownTotal ? `${pct.toFixed(1)}%` : '--'}</b></div>
            <p>{activePage === 'upload' ? '已上传' : '已下载'}&nbsp; <strong>{bytes(snap.CompletedBytes)} / {knownTotal ? bytes(snap.TotalBytes) : '--'}</strong></p>
            <div className="metrics">
              <span>◴ 速度&nbsp; <b>{bytes(snap.Speed)}/s</b>&nbsp;&nbsp; ETA&nbsp; <b>{eta}</b></span>
              <button className="danger" disabled={!running} onClick={() => window.go.main.App.StopTransfer()}><Square size={13}/>停止</button>
            </div>
            {error && <p className="error">{error}</p>}
          </article>

          <article className="stats card"><h2>任务统计</h2><hr/>
            <Stat icon={<Clock3/>} name="待处理" value={String(snap.Pending || 0)}/>
            <Stat icon={<ArrowUpFromLine/>} name="运行" value={String(snap.Running || 0)} blue/>
            <Stat icon={<CircleCheck/>} name="成功" value={String(snap.Succeeded || 0)} good/>
            <Stat icon={<CircleX/>} name="失败" value={String(snap.Failed || 0)} bad/>
            <Stat icon={<X/>} name="取消/跳过" value={`${snap.Canceled || 0}/${snap.Skipped || 0}`}/>
          </article>
        </div>

        <article className="logs card">
          <header><h2>▼ &nbsp;详情 <span>· 实时任务</span></h2><label>{snap.Final ? resultLabel(snap.Status) : running ? '正在更新' : '等待任务'}</label></header><hr/>
          <div className="loghead"><span>状态</span><span>进度</span><span>文件 / 消息</span></div>
          {!items.length && <div className="empty-log"><FolderOpen/><span>开始任务后，这里会显示真实的文件进度和错误。</span></div>}
          {items.map((item) => <div className="log" key={item.TaskID}>
            <b className={`state-${item.Status}`}>{statusLabel(item.Status)}</b>
            <time>{item.TotalBytes > 0 ? `${Math.min(100, item.CompletedBytes * 100 / item.TotalBytes).toFixed(1)}%` : bytes(item.CompletedBytes)}</time>
            <span><strong>{item.FileName || item.SourcePath || item.TaskID}</strong>{item.Err ? ` · ${item.Err}` : item.Phase ? ` · ${item.Phase}` : ''}</span>
          </div>)}
          {(snap.Errors || []).map((message, index) => <div className="log log-error" key={`${message}-${index}`}><b>错误</b><time>—</time><span>{message}</span></div>)}
        </article>
        </>}

        <footer className="status"><span>TDL Desktop · 本地 WebView2</span><span>{bytes(snap.Speed)}/s&nbsp;&nbsp; | &nbsp;&nbsp;任务 {snap.Succeeded + snap.Failed + snap.Canceled}/{Math.max(snap.Succeeded + snap.Failed + snap.Canceled + snap.Pending + snap.Running, items.length)}</span></footer>
      </section>

      {chatOpen && <div className="modal-backdrop" onMouseDown={(event) => event.target === event.currentTarget && setChatOpen(false)}>
        <section className="chat-modal" role="dialog" aria-modal="true" aria-label="选择 Telegram 会话">
          <header><div><small>TELEGRAM DESTINATION</small><h2>选择会话与话题</h2></div><button aria-label="关闭" onClick={() => setChatOpen(false)}><X/></button></header>
          <div className="chat-search"><Search size={18}/><input autoFocus placeholder="搜索已加载的会话、用户名或类型" value={chatQuery} onChange={(event) => setChatQuery(event.target.value)}/></div>
          {chatError && <div className="chat-error">{chatError}</div>}
          <div className="chat-body">
            <div className="chat-list">
              {filteredChats.map((entry) => <button className={sameChat(entry, draftChat) ? 'selected' : ''} key={chatKey(entry)} onClick={() => {setDraftChat(entry); setDraftTopic(null)}}>
                <span className="chat-avatar">{entry.self ? '★' : (entry.title || entry.username || '?').slice(0, 1).toUpperCase()}</span>
                <span><b>{entry.title || entry.username || String(entry.id)}</b><small>{entry.self ? '个人收藏夹' : `${typeLabel(entry.type)}${entry.username ? ` · @${entry.username}` : ''}`}</small></span>
                {sameChat(entry, draftChat) && <Check size={18}/>}
              </button>)}
              {!filteredChats.length && !chatLoading && <p className="no-chat">没有匹配的已加载会话</p>}
              {chatNext > 0 && !chatQuery && <button className="load-more" disabled={chatLoading} onClick={() => loadChats(chatNext)}>{chatLoading ? '加载中…' : '加载更多会话'}</button>}
            </div>
            <aside className="topic-list">
              <small>当前目标</small><h3>{draftChat.title}</h3>
              {draftChat.topics.length ? <>
                <p>该会话包含话题，可选择具体目的地。</p>
                <button className={!draftTopic ? 'selected' : ''} onClick={() => setDraftTopic(null)}><span>#</span><b>不指定话题</b>{!draftTopic && <Check/>}</button>
                {draftChat.topics.map((topic) => <button className={draftTopic?.id === topic.id ? 'selected' : ''} key={topic.id} onClick={() => setDraftTopic(topic)}><span>#</span><b>{topic.title}</b>{draftTopic?.id === topic.id && <Check/>}</button>)}
              </> : <p>此会话没有可选话题，文件会直接发送到会话。</p>}
            </aside>
          </div>
          <footer><span>{chatLoading ? '正在从 Telegram 加载…' : `已加载 ${chatItems.length} 个目标`}</span><div><button onClick={() => setChatOpen(false)}>取消</button><button className="confirm" disabled={chatLoading} onClick={confirmChatSelector}>使用此目标</button></div></footer>
        </section>
      </div>}
    </main>
  );
}

function uniqueChats(items: ChatRef[]) {
  const seen = new Set<string>();
  return items.filter((item) => {
    const key = chatKey(item);
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}
function chatKey(chat: ChatRef) { return chat.self ? 'self' : `${chat.type}:${chat.id}`; }
function sameChat(a: ChatRef, b: ChatRef) { return chatKey(a) === chatKey(b); }
function typeLabel(type: string) { return ({private: '私聊', group: '群组', channel: '频道', self: '收藏夹'} as Record<string, string>)[type] || type; }
function statusLabel(status: string) { return ({queued: '排队', running: '运行', done: '完成', failed: '失败', canceled: '取消', skipped: '跳过', partial_failure: '部分失败'} as Record<string, string>)[status] || status || '等待'; }
function resultLabel(status: string) { return ({done: '任务完成', failed: '任务失败', canceled: '任务已取消', partial_failure: '任务部分失败'} as Record<string, string>)[status] || '任务结束'; }
function errorText(reason: unknown) { return reason instanceof Error ? reason.message : String(reason); }
function bytes(value = 0) { const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']; let size = Number(value) || 0; let index = 0; while (size >= 1024 && index < units.length - 1) { size /= 1024; index++; } return `${size.toFixed(1)} ${units[index]}`; }
function duration(seconds: number) { const value = Math.max(0, Math.round(seconds)); const hours = Math.floor(value / 3600); const minutes = Math.floor((value % 3600) / 60); const rest = value % 60; return hours ? `${hours}h ${minutes}m` : minutes ? `${minutes}m ${rest}s` : `${rest}s`; }
function Stat({icon, name, value, blue, good, bad}: {icon: React.ReactNode; name: string; value: string; blue?: boolean; good?: boolean; bad?: boolean}) { return <div className={`stat ${blue ? 'blue' : ''} ${good ? 'good' : ''} ${bad ? 'bad' : ''}`}>{icon}<span>{name}</span><b>{value}</b></div>; }
