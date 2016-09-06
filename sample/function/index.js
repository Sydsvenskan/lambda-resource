'use strict';
exports.handler = (event, context, callback) => {
  callback(null, {
    event: event,
    context: context,
  });
};
