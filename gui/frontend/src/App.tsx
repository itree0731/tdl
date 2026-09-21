import {ArrowDownToLine, ArrowUpFromLine, Database, MessageSquare, RefreshCcw, Settings, Upload, CircleCheck, CircleX, Clock3, Square} from 'lucide-react';
import {useEffect,useMemo,useState} from 'react';

const nav = [
  [Upload,'上传'],[ArrowDownToLine,'下载'],[MessageSquare,'会话'],[ArrowUpFromLine,'任务'],[Database,'备份'],[RefreshCcw,'恢复'],[Settings,'设置']
] as const;

export function App(){
  const [paths,setPaths]=useState<string[]>([]),[namespaces,setNamespaces]=useState<string[]>([]),[namespace,setNamespace]=useState('default'),[running,setRunning]=useState(false),[error,setError]=useState('');
  const [snap,setSnap]=useState<any>({Pending:0,Running:0,Succeeded:0,Failed:0,Canceled:0,CompletedBytes:0,TotalBytes:0,Speed:0,CurrentFile:'',DiscoveryDone:false});
  useEffect(()=>{window.go?.main.App.Namespaces().then(x=>{setNamespaces(x);if(x.length)setNamespace(x.includes('default')?'default':x[0])}).catch(e=>setError(String(e))); const off1=window.runtime?.EventsOn('transfer:snapshot',setSnap); const off2=window.runtime?.EventsOn('transfer:done',(x:any)=>{setRunning(false);setError(x.error||'')}); return()=>{off1?.();off2?.()}},[]);
  const pct=useMemo(()=>snap.DiscoveryDone&&snap.TotalBytes>0?Math.min(100,snap.CompletedBytes*100/snap.TotalBytes):0,[snap]);
  const pickFiles=async()=>{const x=await window.go.main.App.SelectUploadFiles();if(x?.length)setPaths(x)};
  const pickDir=async()=>{const x=await window.go.main.App.SelectUploadDirectory();if(x)setPaths([x])};
  const start=async()=>{setError('');try{await window.go.main.App.StartUpload({namespace,paths,coverMode:'video-cover',coverAt:'auto',threads:4,limit:2});setRunning(true)}catch(e){setError(String(e))}};
  return <main className="shell">
    <aside className="sidebar">
      <section className="brand"><strong>TDL</strong><span>Telegram Media Transfer</span><em>传 输 工 作 台</em></section>
      <nav>{nav.map(([Icon,label],i)=><button className={i===0?'active':''} key={label}><Icon size={21}/><span>{label}</span></button>)}</nav>
      <footer><div>登录 · 更新 · 版本 · 退出</div><b>让传输更简单</b><small>MEDIA ANYWHERE<br/>WITH TDL</small></footer>
    </aside>
    <section className="workspace">
      <header className="session"><span>当前会话：<b>Workstation</b></span><i/><span>账号：<select value={namespace} onChange={e=>setNamespace(e.target.value)}>{namespaces.map(x=><option key={x}>{x}</option>)}</select></span><i/><span>网络：<mark>● 正常</mark></span><time>{new Date().toLocaleString()}</time></header>
      <div className="overview">
        <article className="media card"><label>MEDIA</label><div className="cover"><span>VIDEO</span><small>720×1280 · 04:21 · H.264</small></div></article>
        <article className="task card">
          <h2><Upload/> 当前任务 <span>· 上传</span></h2><hr/>
          <h1>{snap.CurrentFile||paths[0]?.split(/[\\/]/).pop()||'选择要上传的文件'}</h1><p>{paths.length?`已选择 ${paths.length} 项`:'支持文件和目录 · 默认高清 video_cover'}</p>
          <div className="picker"><button onClick={pickFiles}>选择文件</button><button onClick={pickDir}>选择目录</button><button className="primary" disabled={!paths.length||running} onClick={start}>开始上传</button></div>
          <div className="progress"><span style={{width:`${pct}%`}}/><b>{snap.DiscoveryDone?`${pct.toFixed(1)}%`:'--'}</b></div>
          <p>已上传&nbsp; <strong>{bytes(snap.CompletedBytes)} / {snap.TotalBytes?bytes(snap.TotalBytes):'--'}</strong></p>
          <div className="metrics"><span>◴ 速度&nbsp; <b>{bytes(snap.Speed)}/s</b></span><button>详情</button><button className="danger" disabled={!running} onClick={()=>window.go.main.App.StopTransfer()}><Square size={13}/>停止</button></div>{error&&<p className="error">{error}</p>}
        </article>
        <article className="stats card"><h2>任务统计</h2><hr/><Stat icon={<Clock3/>} name="待处理" value={String(snap.Pending||0)}/><Stat icon={<ArrowUpFromLine/>} name="运行" value={String(snap.Running||0)} blue/><Stat icon={<CircleCheck/>} name="成功" value={String(snap.Succeeded||0)} good/><Stat icon={<CircleX/>} name="失败" value={String(snap.Failed||0)} bad/></article>
      </div>
      <article className="logs card"><header><h2>▼ &nbsp;详情 <span>· 传输日志</span></h2><label>自动滚动&nbsp; <input type="checkbox" defaultChecked/></label></header><hr/>
        <div className="loghead"><span>时间</span><span>级别</span><span>消息</span></div>
        {['开始上传 Alps_4K_Documentary.mp4（4.32 GB）','连接到 Telegram 服务器…','上传会话已建立，开始传输数据','已上传 512 MB（11.4%）  速度 36.8 MB/s','已上传 1.02 GB（23.6%）  速度 38.1 MB/s','已上传 2.96 GB（68.4%）  速度 38.6 MB/s  剩余 00:00:36'].map((x,i)=><div className="log" key={x}><time>14:{25+i*7}:0{i}</time><b>INFO</b><span>{x}</span></div>)}
      </article>
      <footer className="status"><span>[TDL v0.20.4] &nbsp; Telegram Media Transfer</span><span>↑ &nbsp;38.6 MB/s&nbsp;&nbsp; | &nbsp;&nbsp;任务 1/4&nbsp;&nbsp; | &nbsp;&nbsp;CPU 2%&nbsp;&nbsp; | &nbsp;&nbsp;内存 118 MB</span></footer>
    </section>
  </main>
}
function bytes(n:number=0){const u=['B','KiB','MiB','GiB'];let v=Number(n)||0,i=0;while(v>=1024&&i<u.length-1){v/=1024;i++}return `${v.toFixed(1)} ${u[i]}`}
function Stat({icon,name,value,blue,good,bad}:{icon:React.ReactNode,name:string,value:string,blue?:boolean,good?:boolean,bad?:boolean}){return <div className={`stat ${blue?'blue':''} ${good?'good':''} ${bad?'bad':''}`}>{icon}<span>{name}</span><b>{value}</b></div>}
