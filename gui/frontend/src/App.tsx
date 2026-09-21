import {ArrowDownToLine, ArrowUpFromLine, Database, MessageSquare, RefreshCcw, Settings, Upload, CircleCheck, CircleX, Clock3, Square} from 'lucide-react';

const nav = [
  [Upload,'上传'],[ArrowDownToLine,'下载'],[MessageSquare,'会话'],[ArrowUpFromLine,'任务'],[Database,'备份'],[RefreshCcw,'恢复'],[Settings,'设置']
] as const;

export function App(){
  return <main className="shell">
    <aside className="sidebar">
      <section className="brand"><strong>TDL</strong><span>Telegram Media Transfer</span><em>传 输 工 作 台</em></section>
      <nav>{nav.map(([Icon,label],i)=><button className={i===0?'active':''} key={label}><Icon size={21}/><span>{label}</span></button>)}</nav>
      <footer><div>登录 · 更新 · 版本 · 退出</div><b>让传输更简单</b><small>MEDIA ANYWHERE<br/>WITH TDL</small></footer>
    </aside>
    <section className="workspace">
      <header className="session"><span>当前会话：<b>Workstation</b></span><i/><span>账号：<b>default</b></span><i/><span>网络：<mark>● 正常</mark></span><time>2026-09-21&nbsp; 12:30:16</time></header>
      <div className="overview">
        <article className="media card"><label>MEDIA</label><div className="cover"><span>VIDEO</span><small>720×1280 · 04:21 · H.264</small></div></article>
        <article className="task card">
          <h2><Upload/> 当前任务 <span>· 上传</span></h2><hr/>
          <h1>Alps_4K_Documentary.mp4</h1><p>4.32 GB&nbsp;&nbsp; | &nbsp;&nbsp;3840×2160&nbsp;&nbsp; | &nbsp;&nbsp;MP4</p>
          <div className="progress"><span/><b>68.4%</b></div>
          <p>已上传&nbsp; <strong>2.96 GB / 4.32 GB</strong></p>
          <div className="metrics"><span>◴ 速度&nbsp; <b>38.6 MB/s</b></span><span>◷ 剩余时间&nbsp; <b>00:00:36</b></span><button>详情</button><button className="danger"><Square size={13}/>停止</button></div>
        </article>
        <article className="stats card"><h2>任务统计</h2><hr/><Stat icon={<Clock3/>} name="待处理" value="3"/><Stat icon={<ArrowUpFromLine/>} name="运行" value="1" blue/><Stat icon={<CircleCheck/>} name="成功" value="28" good/><Stat icon={<CircleX/>} name="失败" value="2" bad/></article>
      </div>
      <article className="logs card"><header><h2>▼ &nbsp;详情 <span>· 传输日志</span></h2><label>自动滚动&nbsp; <input type="checkbox" defaultChecked/></label></header><hr/>
        <div className="loghead"><span>时间</span><span>级别</span><span>消息</span></div>
        {['开始上传 Alps_4K_Documentary.mp4（4.32 GB）','连接到 Telegram 服务器…','上传会话已建立，开始传输数据','已上传 512 MB（11.4%）  速度 36.8 MB/s','已上传 1.02 GB（23.6%）  速度 38.1 MB/s','已上传 2.96 GB（68.4%）  速度 38.6 MB/s  剩余 00:00:36'].map((x,i)=><div className="log" key={x}><time>14:{25+i*7}:0{i}</time><b>INFO</b><span>{x}</span></div>)}
      </article>
      <footer className="status"><span>[TDL v0.20.4] &nbsp; Telegram Media Transfer</span><span>↑ &nbsp;38.6 MB/s&nbsp;&nbsp; | &nbsp;&nbsp;任务 1/4&nbsp;&nbsp; | &nbsp;&nbsp;CPU 2%&nbsp;&nbsp; | &nbsp;&nbsp;内存 118 MB</span></footer>
    </section>
  </main>
}
function Stat({icon,name,value,blue,good,bad}:{icon:React.ReactNode,name:string,value:string,blue?:boolean,good?:boolean,bad?:boolean}){return <div className={`stat ${blue?'blue':''} ${good?'good':''} ${bad?'bad':''}`}>{icon}<span>{name}</span><b>{value}</b></div>}
