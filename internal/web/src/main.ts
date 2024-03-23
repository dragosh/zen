
import {render} from './md.mjs';

interface Init {
  entryFile?: string;
  rootFile?: string;
  userName?: string;
}


const init: Init = window.__zen__ || {
  entryFile: "",
  rootFile: "./",
  userName: "",
};

if(init.entryFile) render(init.entryFile)
