/** @type {import('tailwindcss').Config} */

const colors = require('tailwindcss/colors')

module.exports = {
  content: ["./ui/web/**/*.tmpl"],
  theme: {
    colors: {
      black: colors.black,
      white: colors.white,
      red: colors.red,
      green: colors.green,
      blue: colors.blue,
    },
  },
  plugins: [],
}