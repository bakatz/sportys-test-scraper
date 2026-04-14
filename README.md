# sportys-test-scraper
<img width="1364" height="211" alt="image" src="https://github.com/user-attachments/assets/2acb4425-d551-4876-8194-7d4afb2e06de" />
<img width="709" height="738" alt="image" src="https://github.com/user-attachments/assets/c97c7467-6084-40d9-81a8-270489afac57" />

# What is This?
Automatically generate new practice tests from your previously missed questions on your past Sporty's ground school / private pilot practice tests (in PDF format). 

Additionally, see statistics like how long you took on each test vs. the allowed time limit on the real test, and how many questions you got wrong per test.

by [@ben_makes_stuff](https://x.com/ben_makes_stuff)

[![Buy me a coffee](https://img.buymeacoffee.com/button-api/?text=Buy%20me%20a%20coffee&emoji=☕&slug=ben_makes_stuff&button_colour=FFDD00&font_colour=000000&font_family=Lato&outline_colour=000000&coffee_colour=ffffff)](https://www.buymeacoffee.com/ben_makes_stuff)

# Usage

1. Download the latest release from the releases page. For recent MacOS devices, pick the download with darwin-arm-64 in the path.
1. Open Chrome Developer Tools (or similar), then navigate to your test prep page for your course, i.e. https://courses.sportys.com/training/portal/demo/course/PRIVATE/testprep. Then go to the requests tab, pick any request like "getList" and look for the "Authorization" header. Your JWT is the long value after the word "Bearer" (do not include Bearer or the space after Bearer)
1. Note the type of course you've taken, possible values are: PRIVATE, INSTRUMENT, etc. The course type is also visible in the above URL ".../demo/course/PRIVATE/..." <-- "PRIVATE"
1. Run ./sportys-test-scraper-whatever-platform -j <your_jwt_token> -c <course_type>
1. See questions_practice.pdf and questions_with_answers.pdf for your auto-generated practice tests, and the output in your terminal for your test taking statistics.

# FAQ
Q: Do I need to be subscribed to Sporty's ground school to use this?
A: No. Their practice test taking tool is completely free (only the course and endorsement cost money), you just need to sign up for a free account on their site to use it.

# Support
No. This is an as-is, MIT-licensed, free and open source release. Fork and make changes yourself if you'd like to.
