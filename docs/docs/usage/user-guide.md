---
title: "User Guide"
sidebar_position: 2
description: "Manage your account and create courses."
---

# User guide

## Your TUM-Live Admin account

In order to manage your own lectures using TUM-Live, you need an account with
administrative access. Please get in touch with us at live@rbg.tum.de to get one.
You'll receive an email with instructions to set a password.
This account will be shared with **all** users who need to edit the course, therefore
we currently recommend requesting a new user on a per-course basis.

If you already have an account, you can start creating your course.

## Create a course

Once you are logged in with your account, you can navigate to the Admin Panel.
On the left-hand side there is a button to create a new course:

![new-course](course-img/new-course.png)

### Course Parameters

This will open a new page where you can set a few parameters of the course:

![img.png](course-img/new-course-prompt.png)

- 1: **TUMOnlineID**: This is optional but very useful. If set you can click the "Load Infos From TUMOnline" button which will fill out some fields like the semester and course name. Additionally, you will be able to:
  - Automatically load the time slots of the course from TUMOnline
  - Make the course available only for users that are enrolled in TUMOnline
  - Show this course more prominently (under "your courses") on the start page to them.
- 2: **Title**: The tile of your course as shown to users.
- 3: **Teaching Term**: When does this course take place? Make sure to format this accordingly (e.g. `Sommersemester 2021` or `Wintersemester 2021/22`)
- 4: **Slug**: This is the identifier for your course. It will show up in the course's URL, should be short and **must** be unique per semester. Example: `Einführung in die Informatik` -> `eidi`
- 5: **Visibility**: Who should be able to see this course? This can be changed later.
  - **Public**: Everyone can view the courses videos, regardless of whether they are enrolled or logged in.
  - **Enrolled**: Users who are either enrolled to your course in TUMOnline or specifically invited by you.
  - **Logged in**: Everyone with a LRZ ID (like `ab12cde`) can log in and see your course
- 6: **Settings**: Some settings for your course. These can be changed later.
  - **Enable VoD**: All streamed lectures will be made public after the stream if this is enabled.
  - **Enable Downloads**: Students will be able to download the lectures. This is highly recommended as it allows students with bad internet connection to participate in the lectures.
  - **Enable Live Chat**: The viewers of this course are able to comment on streams using the live chat. Regardless of the visibility, chat users need to be logged in. You can block people from using the chat if they misbehave.

## Manage lectures

You will now be able to navigate to your course:

![course navigation](course-img/course-nav.png)

If your TUMOnline ID was set, your lectures have been loaded automatically. Otherwise, you can always add lectures on the bottom of the page.
Please add a descriptive Title for your lectures. This is optional but helps your students a lot.
You can also add a description to each stream. You may use Markdown to include links (e.g. to tweedback):

![lecture edit](course-img/lecture-edit.png)
