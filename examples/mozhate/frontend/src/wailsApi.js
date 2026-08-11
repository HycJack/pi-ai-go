// Wails backend binding wrappers
// These call Go functions exposed via Bind in main.go
// Uses the auto-generated bindings from wailsjs/

import * as App from '../wailsjs/go/main/App';

// Get pinyin from Go backend
export async function getPinyin(text) {
    try {
        return await App.GetPinyin(text);
    } catch (e) {
        console.error('GetPinyin error:', e);
        return [];
    }
}

// Generate content via Go backend (AI API)
export async function generateContent(prompt) {
    try {
        return await App.GenerateContent(prompt);
    } catch (e) {
        console.error('GenerateContent error:', e);
        return ['勤*学*苦*练* 积极*向上* 自强*不息*'];
    }
}

// Get AI configuration
export async function getAIConfig() {
    try {
        return await App.GetAIConfig();
    } catch (e) {
        console.error('GetAIConfig error:', e);
        return {
            provider: 'gemini',
            model: 'gemini-2.0-flash',
            apiKey: '',
            endpoint: '',
            promptBase: '',
            maxWords: 15,
            useTracing: true,
        };
    }
}

// Save AI configuration
export async function saveAIConfig(cfg) {
    try {
        return await App.SaveAIConfig(cfg);
    } catch (e) {
        console.error('SaveAIConfig error:', e);
        return false;
    }
}

// Exit the application
export function exitApp() {
    if (window.runtime && window.runtime.Quit) {
        window.runtime.Quit();
    }
}
