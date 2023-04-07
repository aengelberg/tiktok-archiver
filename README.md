# TikTok Archiver

<img src="https://user-images.githubusercontent.com/4122172/228493320-78a140b0-7a25-4ed9-9afc-4a815b93fe84.png" width=300>

:arrow_right: [DOWNLOAD FOR WINDOWS](https://github.com/aengelberg/tiktok-archiver/releases/download/0.0.9/TikTok.Archiver-0.0.9-windows-amd64.zip)

:arrow_right: [DOWNLOAD FOR MAC](https://github.com/aengelberg/tiktok-archiver/releases/download/0.0.9/TikTok.Archiver-0.0.9-macos-amd64.zip)

TikTok Archiver is a tool for batch-archiving all the videos in your TikTok account. It is available to [install](#installing) on Windows and macOS.

It is very different from other third-party TikTok downloading services (SnapTik, SaveTok, etc):

* It processes an archive file that you must first [request and download](https://support.tiktok.com/en/account-and-privacy/personalized-ads-and-data/requesting-your-data) from TikTok.
* It downloads all of the videos from your account, including private and friends-only videos.
* All of the videos will be unwatermarked, at the original resolution and quality that was uploaded to TikTok.

<img width=600 src="https://user-images.githubusercontent.com/4122172/229311721-6d170f0f-c9f3-4162-81b1-58f8a28c2b01.png">

## How to use it

First, you must [request and download](https://support.tiktok.com/en/account-and-privacy/personalized-ads-and-data/requesting-your-data) a full archive of your TikTok account through the TikTok app. You may select either the TXT or JSON format when requesting the data from TikTok.

**This archive usually takes about 3 days to receive after requesting it.** Sadly you must wait until the file is ready before proceeding.

After downloading and unzipping your archive, you will have a file named `Posts.txt` or `user_data.json`, which will contain a list of links to download each of your videos.

Now, in TikTok Archiver:

* Select your Input File by navigating to the `Posts.txt` or `user_data.json` file on your computer.
* Select your Output Directory by navigating to a folder where you'd like all the videos to be downloaded.
* Click "Download" to start the batch download.
* Every video will be saved as an mp4 file to the output directory. The filename of each video will be a timestamp of when the video was posted, e.g. `2022-11-25-04-23-42.mp4`.
* A few videos may fail to download, which is normal. You can look into what happened by clicking "Open Log" and looking for error messages.
* After your batch download is complete, you may retry the failed downloads by clicking "Download" again. By default it will only try to download the videos that aren't already present in the output directory.

# Installing

## macOS

Due to macOS's security safeguards against third-party applications, running TikTok Archiver for the first time takes a couple extra steps.

Download the macOS zip file from the latest [release](https://github.com/aengelberg/tiktok-archiver/releases/latest), then unzip it.

<img width="146" alt="image" src="https://user-images.githubusercontent.com/4122172/228495873-4a83553e-9968-4015-9586-083fb911639b.png">

Double-click on the app to open it. You'll see this warning; press "OK".

<img width="265" alt="Screen Shot 2023-03-31 at 11 13 30 PM" src="https://user-images.githubusercontent.com/4122172/229269100-2202ecdb-5b2a-48e9-b5ba-12699395d7a8.png">

Now, rather than double-clicking on the app, right-click on it in Finder and click "Open".

<img width="329" alt="image" src="https://user-images.githubusercontent.com/4122172/229268919-2efdd37a-4d96-4a61-9b93-a74b8dbed2cc.png">

A security warning will pop up; click "Open" to bypass it.

<img width="275" alt="Screen Shot 2023-03-29 at 2 53 55 AM" src="https://user-images.githubusercontent.com/4122172/229268972-8c3b073d-aad3-49b1-a4a0-1734b0fdd13c.png">

## Windows

Download the Windows zip file from the latest [release](https://github.com/aengelberg/tiktok-archiver/releases/latest), then unzip it. Then, open `TikTok Archiver.exe` to run the application.

Please note that there is a risk of Windows Defender flagging this app as malware and not letting you run it. I'm still working on a workaround for this issue, apologies for the inconvenience.
