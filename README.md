# sportys-test-scraper

# What is This?
Automatically generate new practice tests from your previously missed questions on your past Sporty's practice tests (in PDF format). 

Additionally, see statistics like how long you took on each test vs. the allowed time limit on the real test, and how many questions you got wrong per test.

by [@ben_makes_stuff](https://x.com/ben_makes_stuff)

[![Buy me a coffee](https://img.buymeacoffee.com/button-api/?text=Buy%20me%20a%20coffee&emoji=☕&slug=ben_makes_stuff&button_colour=FFDD00&font_colour=000000&font_family=Lato&outline_colour=000000&coffee_colour=ffffff)](https://www.buymeacoffee.com/ben_makes_stuff)

# Usage

1. Download the latest release from the releases page.
1. Find your JWT by opening Chrome Developer Tools (or similar), then navigate to https://courses.sportys.com/training/portal/demo/course/PRIVATE/testprep. Pick any request like "getList" and look for the "Authorization" header. The JWT is the long value after the word "Bearer" (do not include Bearer or the space after Bearer)
1. Note the type of course you've taken, possible values are: PRIVATE, INSTRUMENT, etc. The course type is also visible in the above URL ".../demo/course/PRIVATE/..." <-- "PRIVATE"
1. Run ./sportys-test-scraper -j <your_jwt_token> -c <course_type>
1. See questions_practice.pdf and questions_with_answers.pdf.

# Support
No. This is an as-is, MIT-licensed, free and open source release. Fork and make changes yourself if you'd like to.
