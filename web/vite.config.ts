import {defineConfig} from 'vite';
import {fileURLToPath,URL} from 'node:url';
export default defineConfig({base:'./',resolve:{alias:{'@':fileURLToPath(new URL('./src',import.meta.url))}},build:{outDir:'../internal/webui/dist',emptyOutDir:true}});
