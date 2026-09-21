import React from 'react';
import {createRoot} from 'react-dom/client';
import {App} from './App';
import './style.css';
import './controls.css';
import './chat.css';
import './tools.css';
import './tools-overrides.css';
createRoot(document.getElementById('root')!).render(<React.StrictMode><App/></React.StrictMode>);
