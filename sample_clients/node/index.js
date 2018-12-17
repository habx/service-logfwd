#!/usr/bin/env node
const winston = require('winston');

//
// Requiring `winston-logstash` will expose
// `winston.transports.Logstash`
//
require('winston-logstash');

winston.add(winston.transports.Logstash, {
    port: 5050,
    node_name: 'mynode',
    host: '127.0.0.1'
});

function logging() {
    winston.info('Hello !')
}

logging();

setInterval(() => {
    logging()
}, 5000);
