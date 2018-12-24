#!/usr/bin/env node
const winston = require('winston');

//
// Requiring `winston-logstash` will expose
// `winston.transports.Logstash`
//
require('winston-logstash');

winston.add(winston.transports.Logstash, {
    host: '127.0.0.1',
    port: 5050,
    node_name: 'mynode',
    meta: {
        appname: 'myapp',
        env: 'dev',
    },
});

let nb = 0;

function logging() {
    winston.info(`Hello ! (${nb++})`)
}

logging();

setInterval(() => {
    logging()
}, 5000);
