'use strict';
exports.handler = (event, context, callback) => {
  // Just echo back all incoming data
  callback(null, {
    event: event,
    context: context
  });
};
