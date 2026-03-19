import React, { useState, useEffect, useRef } from 'react';
import { ArrowDown, Zap, Target, Github, Languages, Star, MessageSquare, X, Triangle, Circle, Hexagon } from 'lucide-react';

const REPO_URL = 'https://github.com/WhitecrowAurora/lulynx-server-monitor';

// --- Content Dictionary ---
const content = {
  en: {
    navTitle: "LULYNX_PROBE.v1",
    chaosOn: "LOUD MODE",
    chaosOff: "FOCUS MODE",
    heroTitle: "LULYNX",
    heroSubtitle1: "SERVER",
    heroSubtitle2: "PROBE",
    heroSubtitle3: "PANEL",
    heroSubtitle4: "CONTROL",
    btnInit: "SEE FEATURES",
    btnPing: "OPEN LINKS",
    profile: "SYSTEM CARD",
    classLabel: "ROLE",
    classVal: "MONITOR",
    alignLabel: "MODEL",
    alignVal: "CENTER+PROBE",
    methodLabel: "DEPLOY",
    methodVal: "SSH",
    marquee: "PUSH METRICS /// ADMIN PANEL /// SILENT REJECT /// ACTIVE OR PASSIVE CONTROL /// LULYNX SERVER PROBE ///",
    coreDump: "FEATURE DROP",
    card1Title: "Deploy Fast",
    card1Desc: "Initialize a center node or a probe with one SSH-friendly script.",
    card1Code: "./run.sh",
    card2Title: "Push Metrics",
    card2Desc: "Track CPU, memory, disk, load, traffic, and optional port status from Linux probes.",
    card2Code: "HTTP PUSH",
    card3Title: "Control Panel",
    card3Desc: "Manage defaults, visibility, node secrets, and active or passive control from one panel.",
    card3Code: "/admin",
    connectTitle: "LINK",
    connectNow: "OUT",
    footer: "OPEN SOURCE /// LULYNX SERVER PROBE /// 2026",
    langSwitch: "CN",
    loadingMessages: [
      "Booting the panel...",
      "Linking probes...",
      "Wiring the center...",
      "Indexing metrics...",
      "Drawing node cards...",
      "Syncing admin routes...",
      "Checking silent reject...",
      "Packing the release..."
    ]
  },
  zh: {
    navTitle: "LULYNX_PROBE.v1",
    chaosOn: "高噪模式",
    chaosOff: "专注模式",
    heroTitle: "LULYNX",
    heroSubtitle1: "服务器",
    heroSubtitle2: "探针",
    heroSubtitle3: "面板",
    heroSubtitle4: "控制",
    btnInit: "看功能",
    btnPing: "开链接",
    profile: "系统卡片",
    classLabel: "角色",
    classVal: "监控",
    alignLabel: "模型",
    alignVal: "中心+节点",
    methodLabel: "部署",
    methodVal: "SSH",
    marquee: "指标推送 /// 控制面板 /// 静默拒绝 /// 主动/被动控制 /// LULYNX SERVER PROBE ///",
    coreDump: "功能速览",
    card1Title: "快速部署",
    card1Desc: "用一套 SSH 交互脚本初始化中心端或客户端。",
    card1Code: "./run.sh",
    card2Title: "指标上报",
    card2Desc: "采集 CPU、内存、磁盘、负载、流量，以及可选端口探测信息。",
    card2Code: "HTTP PUSH",
    card3Title: "控制面板",
    card3Desc: "在一个面板里管理默认策略、节点显示、节点密码和主动/被动模式。",
    card3Code: "/admin",
    connectTitle: "项目",
    connectNow: "入口",
    footer: "开源项目 /// LULYNX SERVER PROBE /// 2026",
    langSwitch: "EN",
    loadingMessages: [
      "正在启动面板...",
      "正在连接探针...",
      "正在装配中心端...",
      "正在索引指标...",
      "正在绘制节点卡片...",
      "正在同步管理路由...",
      "正在检查静默拒绝...",
      "正在打包发布..."
    ]
  }
};

// --- Helper Components ---

// Text Scramble Component (Data Decode Effect)
const ScrambleText = ({ text, className = '', hoverTrigger = true, autoStart = false }) => {
  const [display, setDisplay] = useState(text);
  const chars = '!@#$%^&*()_+-=[]{}|;:,.<>?/0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ';
  const intervalRef = useRef(null);

  const scramble = () => {
    let iteration = 0;
    clearInterval(intervalRef.current);

    intervalRef.current = setInterval(() => {
      setDisplay(
        text
          .split("")
          .map((char, index) => {
            if (index < iteration) return text[index];
            return chars[Math.floor(Math.random() * chars.length)];
          })
          .join("")
      );

      if (iteration >= text.length) {
        clearInterval(intervalRef.current);
      }

      iteration += 1 / 2; // Speed
    }, 30);
  };

  useEffect(() => {
    if (autoStart) scramble();
  }, [text, autoStart]);

  useEffect(() => {
    // Initial scramble on mount/text change
    scramble();
    return () => clearInterval(intervalRef.current);
  }, [text]);

  return (
    <span 
      className={`inline-block ${className}`} 
      onMouseEnter={hoverTrigger ? scramble : undefined}
    >
      {display}
    </span>
  );
};

// Click Popups (Comic "POW!" Effect)
const ClickBang = () => {
  const [bangs, setBangs] = useState([]);
  const words = ["PING!", "PUSH!", "NODE!", "PANEL!", "SYNC!", "HTTP!", "OSS!", "UP!", "LIVE!"];
  const colors = ["#FF00FF", "#00FFFF", "#FFFF00", "#FF0000", "#000000", "#FFFFFF"];

  useEffect(() => {
    const handleClick = (e) => {
      const id = Date.now();
      const word = words[Math.floor(Math.random() * words.length)];
      const color = colors[Math.floor(Math.random() * colors.length)];
      const rotation = Math.random() * 40 - 20; // -20 to 20 deg

      setBangs(prev => [...prev, { id, x: e.clientX, y: e.clientY, word, color, rotation }]);

      // Remove after animation
      setTimeout(() => {
        setBangs(prev => prev.filter(b => b.id !== id));
      }, 800);
    };

    window.addEventListener('mousedown', handleClick);
    return () => window.removeEventListener('mousedown', handleClick);
  }, []);

  return (
    <div className="fixed inset-0 pointer-events-none z-[9999] overflow-hidden">
      {bangs.map(bang => (
        <div 
          key={bang.id}
          className="absolute font-black text-2xl border-[3px] border-black px-2 py-1 shadow-[4px_4px_0px_rgba(0,0,0,0.5)] animate-bang-pop"
          style={{ 
            left: bang.x, 
            top: bang.y, 
            backgroundColor: bang.color,
            color: bang.color === '#000000' ? 'white' : 'black',
            transform: `translate(-50%, -50%) rotate(${bang.rotation}deg)`
          }}
        >
          {bang.word}
        </div>
      ))}
    </div>
  );
};

// Preloader Component
const Preloader = ({ messages, onComplete }) => {
  const [progress, setProgress] = useState(0);
  const [currentMessage, setCurrentMessage] = useState(messages[0]);
  const [isFinished, setIsFinished] = useState(false);

  useEffect(() => {
    const interval = setInterval(() => {
      setProgress(prev => {
        const next = prev + Math.random() * 5;
        if (next >= 100) {
          clearInterval(interval);
          setTimeout(() => setIsFinished(true), 500); // Wait a bit at 100%
          setTimeout(onComplete, 1000); // Trigger unmount
          return 100;
        }
        return next;
      });
    }, 100);

    // Message cycler
    const msgInterval = setInterval(() => {
      setCurrentMessage(messages[Math.floor(Math.random() * messages.length)]);
    }, 600);

    return () => {
      clearInterval(interval);
      clearInterval(msgInterval);
    };
  }, [messages, onComplete]);

  return (
    <div 
      className={`fixed inset-0 z-[10000] bg-[#FFFF00] flex flex-col items-center justify-center transition-transform duration-500 ease-in-out ${isFinished ? '-translate-y-full' : 'translate-y-0'}`}
    >
      <div className="w-full max-w-md px-8">
        <div className="flex justify-between font-black text-4xl mb-4 uppercase tracking-tighter">
          <span>LOADING</span>
          <span>{Math.floor(progress)}%</span>
        </div>
        
        {/* Brutalist Progress Bar */}
        <div className="w-full h-8 border-[4px] border-black bg-white p-1 mb-8 shadow-[8px_8px_0px_black]">
          <div 
            className="h-full bg-black transition-all duration-100" 
            style={{ width: `${progress}%` }}
          ></div>
        </div>

        {/* Quirky Message */}
        <div className="text-center font-mono font-bold text-xl border-[2px] border-black bg-white inline-block px-4 py-2 transform -rotate-1 shadow-[4px_4px_0px_rgba(0,0,0,0.2)]">
          <ScrambleText text={currentMessage} autoStart={true} hoverTrigger={false} />
        </div>
      </div>

      {/* Decorative background shapes for loader */}
      <div className="absolute top-10 left-10 animate-bounce-slow">
        <Star size={64} fill="black" />
      </div>
      <div className="absolute bottom-10 right-10 animate-bounce-delayed">
        <X size={80} strokeWidth={4} />
      </div>
    </div>
  );
};

const HalftoneBackground = () => (
  <div className="fixed inset-0 z-0 pointer-events-none opacity-20"
    style={{
      backgroundImage: `radial-gradient(#333 1.5px, transparent 1.5px)`,
      backgroundSize: '20px 20px'
    }}
  ></div>
);

// Graffiti / Decor Background - Added Parallax support
const GraffitiBackground = ({ chaosMode, mousePos }) => {
  // Simple parallax calculation
  const pX = (factor) => (mousePos.x * factor) / 50;
  const pY = (factor) => (mousePos.y * factor) / 50;

  return (
    <div className="fixed inset-0 z-0 pointer-events-none overflow-hidden select-none mix-blend-multiply transition-transform duration-75 ease-out">
      {/* Giant Watermark */}
      <div 
        className={`absolute -right-20 top-20 text-[40vh] font-black text-[#ccc] opacity-40 leading-none transform -rotate-12 transition-transform duration-500 ${chaosMode ? 'scale-110' : 'scale-100'}`}
        style={{ transform: `translate(${pX(-1)}px, ${pY(-1)}px) rotate(-12deg)` }}
      >
        LULYNX
      </div>
      
      {/* Random Shapes with Parallax */}
      <div className={`absolute top-[15%] left-[5%] opacity-30 ${chaosMode ? 'animate-shake' : ''}`} style={{ transform: `translate(${pX(2)}px, ${pY(2)}px) rotate(-12deg)`, animationDelay: '0.5s' }}>
        <Star size={120} strokeWidth={1} fill="none" stroke="black" />
      </div>
      <div className={`absolute top-[60%] right-[10%] opacity-20 text-[#FF00FF] ${chaosMode ? 'animate-shake' : ''}`} style={{ transform: `translate(${pX(-3)}px, ${pY(-3)}px) rotate(45deg)`, animationDelay: '1.2s' }}>
        <Zap size={200} strokeWidth={4} />
      </div>
      <div className={`absolute bottom-[10%] left-[15%] opacity-20 text-[#00FFFF] ${chaosMode ? 'animate-shake' : ''}`} style={{ transform: `translate(${pX(1.5)}px, ${pY(1.5)}px) rotate(12deg)`, animationDelay: '0.8s' }}>
        <Triangle size={150} strokeWidth={4} fill="none" />
      </div>
      <div className={`absolute top-[30%] right-[30%] opacity-10 ${chaosMode ? 'animate-shake' : ''}`} style={{ transform: `translate(${pX(-2)}px, ${pY(-2)}px)`, animationDelay: '0.2s' }}>
        <Circle size={80} strokeWidth={8} stroke="black" fill="none" />
      </div>
      <div className={`absolute top-[80%] left-[40%] opacity-15 ${chaosMode ? 'animate-shake' : ''}`} style={{ transform: `translate(${pX(3)}px, ${pY(3)}px) rotate(6deg)`, animationDelay: '1.5s' }}>
        <X size={100} strokeWidth={8} className="text-black" />
      </div>

      {/* Scribbles */}
      <svg className={`absolute top-[20%] left-[20%] w-64 h-64 opacity-20 ${chaosMode ? 'animate-shake' : ''}`} style={{ transform: `translate(${pX(1)}px, ${pY(1)}px) rotate(-12deg)`, animationDelay: '0.3s' }} viewBox="0 0 100 100">
        <path d="M10,50 Q30,10 50,50 T90,50" fill="none" stroke="black" strokeWidth="2" />
      </svg>
      <svg className={`absolute bottom-[20%] right-[40%] w-96 h-96 opacity-10 ${chaosMode ? 'animate-shake' : ''}`} style={{ transform: `translate(${pX(-1.5)}px, ${pY(-1.5)}px)`, animationDelay: '0.9s' }} viewBox="0 0 200 200">
        <circle cx="100" cy="100" r="80" fill="none" stroke="black" strokeWidth="1" strokeDasharray="10 10" />
      </svg>
    </div>
  );
};

const PrintGlitchText = ({ text, className = '', intense = false }) => {
  return (
    <div className={`relative inline-block font-black ${className}`}>
      {/* Cyan Layer */}
      <span className={`absolute top-0 left-0 text-[#00FFFF] mix-blend-multiply transition-transform duration-200 ${intense ? '-translate-x-2 -translate-y-1' : '-translate-x-1'}`}>
        <ScrambleText text={text} hoverTrigger={false} />
      </span>
      {/* Magenta Layer */}
      <span className={`absolute top-0 left-0 text-[#FF00FF] mix-blend-multiply transition-transform duration-200 ${intense ? 'translate-x-2 translate-y-1' : 'translate-x-1'}`}>
        <ScrambleText text={text} hoverTrigger={false} />
      </span>
      {/* Yellow Layer */}
      <span className={`absolute top-0 left-0 text-[#FFFF00] mix-blend-multiply transition-transform duration-200 ${intense ? '-translate-y-2' : 'translate-y-1'}`}>
        <ScrambleText text={text} hoverTrigger={false} />
      </span>
      {/* Black Key Layer */}
      <span className="relative z-10 text-black cursor-crosshair">
        <ScrambleText text={text} />
      </span>
    </div>
  );
};

const ComicButton = ({ children, onClick, color = 'bg-white text-black', className = '', chaosMode, delay = '0s' }) => (
  <button
    onClick={onClick}
    className={`
      relative group px-8 py-4 font-black border-[3px] border-black ${color}
      shadow-[6px_6px_0px_0px_rgba(0,0,0,1)]
      hover:-translate-y-1 hover:-translate-x-1 hover:shadow-[10px_10px_0px_0px_rgba(0,0,0,1)]
      active:translate-y-0 active:translate-x-0 active:shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
      transition-all duration-150 uppercase tracking-wider text-xl transform hover:rotate-1
      ${className}
      ${chaosMode ? 'animate-shake' : ''}
    `}
    style={{ animationDelay: delay }}
  >
    {children}
  </button>
);

const SpeechBubble = ({ children, direction = 'left', bgColor = 'bg-white', className = '', chaosMode, delay = '0s' }) => {
  const fillHex = bgColor.includes('black') ? '#000000' : '#FFFFFF';
  
  return (
    <div 
      className={`relative border-[3px] border-black p-4 shadow-[4px_4px_0px_black] ${bgColor} ${className} ${chaosMode ? 'animate-shake' : ''}`}
      style={{ animationDelay: delay }}
    >
      {children}
      <svg 
        width="24" 
        height="24" 
        viewBox="0 0 24 24" 
        className={`absolute -bottom-[23px] ${direction === 'left' ? 'left-6' : 'right-6 scale-x-[-1]'} z-10`}
        style={{ overflow: 'visible' }} 
      >
        <path 
          d="M0 0 L12 20 L24 0" 
          fill={fillHex} 
          stroke="black" 
          strokeWidth="3" 
          strokeLinejoin="round" 
          transform="translate(0, -2.5)"
        />
        <rect x="3" y="-4" width="18" height="5" fill={fillHex} />
      </svg>
    </div>
  );
};

const Sticker = ({ children, rotation = 0, color = 'bg-[#FFFF00]', className, chaosMode, delay = '0s' }) => (
  <div 
    className={`absolute z-20 border-[3px] border-black ${color} text-black px-3 py-1 font-black text-sm uppercase shadow-[3px_3px_0px_rgba(0,0,0,0.2)] ${className} ${chaosMode ? 'animate-shake' : ''}`}
    style={{ transform: `rotate(${rotation}deg)`, animationDelay: delay }}
  >
    {children}
  </div>
);

// --- Main App ---

const App = () => {
  const [loading, setLoading] = useState(true);
  const [chaosMode, setChaosMode] = useState(false);
  const [lang, setLang] = useState('zh');
  const [mousePos, setMousePos] = useState({ x: 0, y: 0 });
  const t = content[lang];

  useEffect(() => {
    const handleMouseMove = (e) => setMousePos({ x: e.clientX, y: e.clientY });
    window.addEventListener('mousemove', handleMouseMove);
    return () => window.removeEventListener('mousemove', handleMouseMove);
  }, []);

  const toggleChaos = () => {
    setChaosMode(!chaosMode);
    if (navigator.vibrate) navigator.vibrate(50);
  };

  return (
    <div className={`min-h-screen bg-[#D4D4D8] font-sans text-black overflow-x-hidden selection:bg-black selection:text-[#FFFF00] ${chaosMode ? 'chaos-mode' : ''}`}>
      
      {/* Prank Loader */}
      {loading && (
        <Preloader 
          messages={t.loadingMessages} 
          onComplete={() => setLoading(false)} 
        />
      )}

      {/* Comic Bang Effects */}
      <ClickBang />
      
      <HalftoneBackground />
      <GraffitiBackground chaosMode={chaosMode} mousePos={mousePos} />

      {/* Comic Cursor */}
      <div 
        className="fixed pointer-events-none z-[100] hidden md:block transition-transform duration-100 ease-out will-change-transform mix-blend-multiply"
        style={{ 
          left: mousePos.x, 
          top: mousePos.y, 
          transform: `translate(-50%, -50%) scale(${chaosMode ? 1.5 : 1})` 
        }}
      >
        <Target size={48} strokeWidth={3} className="text-black" />
      </div>

      {/* Navigation: Sticker Bar */}
      <nav className="fixed top-0 w-full z-50 p-4 flex justify-between items-start pointer-events-none">
        <div 
          className={`bg-[#FFFF00] border-[3px] border-black px-4 py-2 shadow-[4px_4px_0px_black] transform -rotate-1 pointer-events-auto hover:rotate-0 transition-transform cursor-pointer ${chaosMode ? 'animate-shake' : ''}`}
          style={{ animationDelay: '0.1s' }}
        >
          <span className="font-black text-2xl tracking-tighter italic inline-block">
             <ScrambleText text={t.navTitle} />
          </span>
        </div>
        
        <div className="flex gap-4 pointer-events-auto">
           <button 
             onClick={() => setLang(l => l === 'en' ? 'zh' : 'en')}
             className={`bg-white px-3 py-2 font-bold border-[3px] border-black shadow-[4px_4px_0px_black] hover:translate-y-1 hover:shadow-none transition-all flex items-center gap-2 transform rotate-2 ${chaosMode ? 'animate-shake' : ''}`}
             style={{ animationDelay: '0.3s' }}
           >
             <Languages size={20} strokeWidth={3} /> {t.langSwitch}
           </button>
           <button 
             onClick={toggleChaos}
             className={`px-4 py-2 font-black border-[3px] border-black shadow-[4px_4px_0px_black] uppercase transition-all transform hover:-rotate-1
               ${chaosMode ? 'bg-[#FF00FF] text-white animate-pulse animate-shake' : 'bg-[#00FFFF]'}`}
             style={{ animationDelay: '0s' }}
           >
             <ScrambleText text={chaosMode ? t.chaosOn : t.chaosOff} />
           </button>
        </div>
      </nav>

      {/* HERO: The Cover Page */}
      <header className="relative min-h-[100svh] flex flex-col justify-center items-center px-4 overflow-hidden pt-10 pb-32">
        
        {/* Decorative Elements */}
        <div className={`absolute top-20 left-10 w-32 h-32 bg-[#FF00FF] rounded-full mix-blend-multiply opacity-80 animate-bounce-slow z-10 ${chaosMode ? 'animate-shake' : ''}`}></div>
        <div className={`absolute bottom-20 right-10 w-40 h-40 bg-[#00FFFF] rounded-full mix-blend-multiply opacity-80 animate-bounce-delayed z-10 ${chaosMode ? 'animate-shake' : ''}`}></div>
        
        <div className="relative z-10 max-w-6xl w-full text-center">
          
          <Sticker rotation={-12} color="bg-[#FF0055] text-white" className="top-0 left-[5%] md:left-[20%]" chaosMode={chaosMode} delay="0.4s">OSS</Sticker>
          <Sticker rotation={15} color="bg-[#00FFFF]" className="bottom-20 right-[5%] md:right-[20%]" chaosMode={chaosMode} delay="1.1s">PUSH</Sticker>

          <div className={`relative inline-block mb-8 ${chaosMode ? 'animate-shake' : ''}`} style={{ animationDelay: '0.2s' }}>
            <PrintGlitchText 
              text={t.heroTitle} 
              className="text-[18vw] md:text-[14vw] leading-[0.8] tracking-tighter" 
              intense={chaosMode} 
            />
            {/* Underline Scribble */}
            <svg className="absolute w-full h-12 -bottom-4 left-0 text-black pointer-events-none" viewBox="0 0 200 20" preserveAspectRatio="none">
              <path d="M0,10 Q50,20 100,5 T200,10" fill="none" stroke="currentColor" strokeWidth="8" />
            </svg>
          </div>

          <div className="flex flex-col md:flex-row justify-center items-center gap-8 md:gap-16 mb-12">
            {/* Left Bubble: White */}
            <SpeechBubble direction="right" bgColor="bg-white" className="transform -rotate-2 max-w-xs text-left" chaosMode={chaosMode} delay="0.5s">
              <p className="font-black text-2xl uppercase italic leading-none">
                {t.heroSubtitle1} <span className="bg-[#FFFF00] px-1 border-2 border-black inline-block transform -rotate-1">{t.heroSubtitle2}</span>
              </p>
            </SpeechBubble>

            {/* Right Bubble: Black */}
            <SpeechBubble direction="left" bgColor="bg-black" className="transform rotate-3 max-w-xs text-left" chaosMode={chaosMode} delay="0.7s">
              <p className="font-black text-2xl uppercase italic leading-none text-[#FFFF00]">
                {t.heroSubtitle3} <span className="text-[#00FFFF] bg-black px-1 border border-[#00FFFF]">{t.heroSubtitle4}</span>
              </p>
            </SpeechBubble>
          </div>

          <div className="flex gap-6 justify-center flex-wrap">
            <ComicButton onClick={() => document.getElementById('content').scrollIntoView({ behavior: 'smooth' })} color="bg-[#FFFF00] text-black" chaosMode={chaosMode} delay="0.1s">
              {t.btnInit} <ArrowDown className="inline ml-2" strokeWidth={3} />
            </ComicButton>
            <ComicButton onClick={() => document.getElementById('contact').scrollIntoView({ behavior: 'smooth' })} color="bg-[#FF0055] text-white" chaosMode={chaosMode} delay="0.6s">
              {t.btnPing} <Zap className="inline ml-2 fill-[#FFFF00] text-[#FFFF00]" />
            </ComicButton>
          </div>

        </div>

        {/* Character Card Floating */}
        <div className={`hidden lg:block absolute right-12 top-1/2 transform -translate-y-1/2 rotate-6 z-20 ${chaosMode ? 'animate-shake' : ''}`} style={{ animationDelay: '0.9s' }}>
          <div className="bg-white border-[3px] border-black p-4 shadow-[8px_8px_0px_black] w-64">
             <div className="bg-black text-white font-black text-center py-1 mb-2 uppercase">
                <ScrambleText text={t.profile} />
             </div>
             <div className="space-y-2 font-bold font-mono text-sm">
                <div className="flex justify-between border-b-2 border-dashed border-gray-300 pb-1">
                  <span>{t.classLabel}</span> <span className="text-[#FF0055]"><ScrambleText text={t.classVal} /></span>
                </div>
                <div className="flex justify-between border-b-2 border-dashed border-gray-300 pb-1">
                  <span>{t.alignLabel}</span> <span className="text-[#00FFFF]"><ScrambleText text={t.alignVal} /></span>
                </div>
                <div className="flex justify-between">
                  <span>{t.methodLabel}</span> <span className="bg-[#FFFF00] px-1"><ScrambleText text={t.methodVal} /></span>
                </div>
             </div>
             <div className="mt-4 flex gap-2 justify-center">
               <Star size={16} fill="black" />
               <Star size={16} fill="black" />
               <Star size={16} fill="black" />
               <Star size={16} fill="black" />
               <Star size={16} />
             </div>
          </div>
        </div>
        
        {/* MARQUEE TAPE - Lifted Higher */}
        <div 
          className={`absolute bottom-24 w-full bg-black text-[#FFFF00] py-3 border-y-[4px] border-black overflow-hidden transform -rotate-1 z-30 shadow-[0px_4px_10px_rgba(0,0,0,0.2)] ${chaosMode ? 'animate-shake' : ''}`}
          style={{ animationDelay: '0.15s' }}
        >
          <div className="whitespace-nowrap animate-marquee font-black text-3xl tracking-widest flex gap-8 italic">
            <span>{t.marquee}</span>
            <span>{t.marquee}</span>
            <span>{t.marquee}</span>
          </div>
        </div>

      </header>

      {/* CONTENT GRID */}
      <section id="content" className="py-24 px-4 relative">
         {/* Decorative background shape */}
         <div className="absolute inset-0 bg-white/50 skew-y-2 z-0 transform -translate-y-10 border-t-[4px] border-black"></div>

        <div className="max-w-7xl mx-auto relative z-10">
          
          <div className="mb-16 text-center">
            <h2 
              className={`text-6xl md:text-8xl font-black uppercase italic tracking-tighter text-transparent stroke-black relative inline-block ${chaosMode ? 'animate-shake' : ''}`}
              style={{ WebkitTextStroke: '3px black', animationDelay: '0.4s' }}
            >
              <ScrambleText text={t.coreDump} />
              <Sticker rotation={10} color="bg-[#00FFFF]" className="-top-4 -right-8" chaosMode={chaosMode} delay="1.2s">HOT!</Sticker>
            </h2>
          </div>

          <div className="grid md:grid-cols-3 gap-8 md:gap-12 px-4">
            
            {/* Comic Panel 1 */}
            <div className={`border-[4px] border-black p-0 shadow-[8px_8px_0px_#FF0055] bg-white transition-transform hover:-translate-y-2 hover:rotate-1 group ${chaosMode ? 'animate-shake' : ''}`} style={{ animationDelay: '0s' }}>
              <div className="bg-black text-white p-3 font-black text-xl uppercase border-b-[4px] border-black flex justify-between items-center">
                <ScrambleText text={t.card1Title} /> <Hexagon className="group-hover:rotate-12 transition-transform" />
              </div>
              <div className="p-6">
                <div className="w-full h-32 bg-[#F0F0F0] mb-4 border-[3px] border-black flex items-center justify-center overflow-hidden relative">
                   {/* Abstract Art */}
                   <div className="absolute w-20 h-20 bg-[#FF0055] rounded-full mix-blend-multiply left-4"></div>
                   <div className="absolute w-20 h-20 bg-[#00FFFF] rounded-full mix-blend-multiply right-4"></div>
                </div>
                <p className="font-bold text-lg mb-4 leading-tight">"{t.card1Desc}"</p>
                <div className="font-mono bg-[#FFFF00] p-2 border-[2px] border-black text-xs font-bold inline-block transform -rotate-2">
                  {t.card1Code}
                </div>
              </div>
            </div>

            {/* Comic Panel 2 */}
            <div className={`border-[4px] border-black p-0 shadow-[8px_8px_0px_#00FFFF] bg-white transition-transform hover:-translate-y-2 hover:-rotate-1 mt-8 md:mt-0 ${chaosMode ? 'animate-shake' : ''}`} style={{ animationDelay: '0.25s' }}>
              <div className="bg-white text-black p-3 font-black text-xl uppercase border-b-[4px] border-black flex justify-between items-center">
                <ScrambleText text={t.card2Title} /> <Zap className="fill-[#FFFF00]" />
              </div>
              <div className="p-6 relative overflow-hidden">
                {/* Speed Lines Background */}
                <div className="absolute inset-0 opacity-10" style={{ backgroundImage: 'repeating-linear-gradient(45deg, black, black 1px, transparent 1px, transparent 10px)' }}></div>
                
                <p className="font-bold text-lg mb-4 leading-tight relative z-10">"{t.card2Desc}"</p>
                <div className="font-mono bg-black text-white p-2 border-[2px] border-black text-xs font-bold inline-block transform rotate-1">
                  {t.card2Code}
                </div>
              </div>
            </div>

            {/* Comic Panel 3 */}
            <div className={`border-[4px] border-black p-0 shadow-[8px_8px_0px_#FFFF00] bg-white transition-transform hover:-translate-y-2 hover:rotate-1 md:mt-16 ${chaosMode ? 'animate-shake' : ''}`} style={{ animationDelay: '0.5s' }}>
              <div className="bg-[#FF0055] text-white p-3 font-black text-xl uppercase border-b-[4px] border-black flex justify-between items-center">
                <ScrambleText text={t.card3Title} /> <Target />
              </div>
              <div className="p-6">
                 <p className="font-bold text-lg mb-4 leading-tight">"{t.card3Desc}"</p>
                 <div className="font-mono bg-[#00FFFF] p-2 border-[2px] border-black text-xs font-bold inline-block transform -rotate-1">
                  {t.card3Code}
                </div>
              </div>
            </div>

          </div>
        </div>
      </section>

      {/* CONTACT SECTION */}
      <section id="contact" className="py-24 px-4 bg-[#FFFF00] border-t-[4px] border-black relative overflow-hidden">
        {/* Halftone Overlay */}
        <div className="absolute inset-0 pointer-events-none opacity-20"
          style={{ backgroundImage: `radial-gradient(#000 2px, transparent 2px)`, backgroundSize: '15px 15px' }}
        ></div>

        <div className="max-w-5xl mx-auto relative z-10 text-center">
          
          <div className="mb-12 transform -rotate-2">
            <h2 className={`text-[15vw] md:text-[10vw] leading-[0.8] font-black uppercase text-black drop-shadow-[6px_6px_0px_rgba(255,255,255,1)] ${chaosMode ? 'animate-shake' : ''}`} style={{ animationDelay: '0.2s' }}>
              {t.connectTitle} <span className="text-white text-transparent stroke-black" style={{ WebkitTextStroke: '3px black' }}>{t.connectNow}</span>
            </h2>
          </div>

          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
             {[
               { icon: Github, label: "GITHUB", color: "bg-[#FF0055] text-white", href: REPO_URL, delay: '0.1s' },
               { icon: MessageSquare, label: "ISSUES", color: "bg-[#00FFFF] text-black", href: `${REPO_URL}/issues`, delay: '0.3s' },
               { icon: Star, label: "README", color: "bg-white text-black", href: `${REPO_URL}#readme`, delay: '0.5s' },
               { icon: ArrowDown, label: "RELEASES", color: "bg-black text-white", href: `${REPO_URL}/releases`, delay: '0.7s' },
              ].map((item, i) => (
               <a key={i} href={item.href} target="_blank" rel="noreferrer"
                  className={`
                    group border-[3px] border-black p-6 ${item.color} shadow-[6px_6px_0px_black]
                    hover:translate-x-1 hover:translate-y-1 hover:shadow-none transition-all
                   flex flex-col items-center justify-center gap-2 aspect-square
                   ${chaosMode ? 'animate-shake' : ''}
                 `}
                 style={{ animationDelay: item.delay }}
               >
                 <item.icon size={48} strokeWidth={2.5} className="group-hover:scale-110 transition-transform" />
                 <span className="font-black text-xl tracking-tighter"><ScrambleText text={item.label} /></span>
               </a>
             ))}
          </div>

          <div 
            className={`mt-16 font-mono font-bold text-sm bg-white inline-block px-4 py-2 border-[2px] border-black transform rotate-1 ${chaosMode ? 'animate-shake' : ''}`}
            style={{ animationDelay: '1s' }}
          >
             <ScrambleText text={t.footer} />
          </div>

        </div>
      </section>

      {/* CSS Utilities */}
      <style>{`
        .animate-bounce-slow { animation: bounce 3s infinite; }
        .animate-bounce-delayed { animation: bounce 3s infinite 1.5s; }
        
        @keyframes shake {
          0% { transform: translate(1px, 1px) rotate(0deg); }
          10% { transform: translate(-1px, -2px) rotate(-1deg); }
          20% { transform: translate(-3px, 0px) rotate(1deg); }
          30% { transform: translate(3px, 2px) rotate(0deg); }
          40% { transform: translate(1px, -1px) rotate(1deg); }
          50% { transform: translate(-1px, 2px) rotate(-1deg); }
          60% { transform: translate(-3px, 1px) rotate(0deg); }
          70% { transform: translate(3px, 1px) rotate(-1deg); }
          80% { transform: translate(-1px, -1px) rotate(1deg); }
        }
        .animate-shake {
          animation: shake 0.5s cubic-bezier(.36,.07,.19,.97) both infinite;
        }

        /* Comic "BANG" Popup Animation */
        @keyframes bang-pop {
          0% { transform: translate(-50%, -50%) scale(0.5) rotate(-10deg); opacity: 0; }
          40% { transform: translate(-50%, -50%) scale(1.2) rotate(10deg); opacity: 1; }
          60% { transform: translate(-50%, -50%) scale(1) rotate(0deg); opacity: 1; }
          100% { transform: translate(-50%, -80%) scale(1.1) rotate(-5deg); opacity: 0; }
        }
        .animate-bang-pop {
          animation: bang-pop 0.8s ease-out forwards;
        }

        .chaos-mode {
          filter: contrast(1.2) saturate(1.2);
        }
        
        @keyframes marquee {
          0% { transform: translateX(0); }
          100% { transform: translateX(-50%); }
        }
        .animate-marquee { animation: marquee 10s linear infinite; }
      `}</style>
    </div>
  );
};

export default App;
