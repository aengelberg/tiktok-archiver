# TikTok Archiver

<img src="https://user-images.githubusercontent.com/4122172/228493320-78a140b0-7a25-4ed9-9afc-4a815b93fe84.png" width=300>

TikTok Archiver is a tool for batch-archiving all the videos in your TikTok account. It is available to [install](#installing) on Windows and macOS.

It is very different from other third-party TikTok downloading services (SnapTik, SaveTok, etc):

* It processes an archive file that you must first [request and download](https://support.tiktok.com/en/account-and-privacy/personalized-ads-and-data/requesting-your-data) from TikTok.
* It downloads all of the videos from your account, including private and friends-only videos.
* All of the videos will be unwatermarked, at the original resolution and quality that was uploaded to TikTok.

<img src="https://user-images.githubusercontent.com/4122172/228669236-d127540c-76ef-4b80-8156-0000158e4227.png" width=500>

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

## Windows

Download the Windows zip file from the latest [release](https://github.com/aengelberg/tiktok-archiver/releases/latest), then unzip it. Then, open `TikTok Archiver.exe` to run the application.

## macOS

Due to macOS's security safeguards against third-party applications, running TikTok Archiver for the first time takes a few extra steps.

Download the macOS zip file from the latest [release](https://github.com/aengelberg/tiktok-archiver/releases/latest), then unzip it.

<img width="146" alt="image" src="https://user-images.githubusercontent.com/4122172/228495873-4a83553e-9968-4015-9586-083fb911639b.png">

Double-click on the TikTok Archiver app to open it.

<img width="136" alt="image" src="https://user-images.githubusercontent.com/4122172/228496157-db785d98-9219-4283-aec7-9c791b00039e.png">

The first time you try to open it, this security warning will appear. Click "Cancel".

<img width="286" alt="Screen Shot 2023-03-29 at 2 51 02 AM" src="https://user-images.githubusercontent.com/4122172/228496823-3cbc204b-b0c7-4120-86dc-43efa547b037.png">

In the "Security & Privacy" section of your Settings app, click "Open Anyway".

<img width="500" alt="Screen Shot 2023-03-29 at 2 52 29 AM" src="https://user-images.githubusercontent.com/4122172/228497182-a6484515-95e2-4e70-b64e-f69c61d1dd7b.png">

Finally, click "Open".

<img width="275" alt="Screen Shot 2023-03-29 at 2 53 55 AM" src="https://user-images.githubusercontent.com/4122172/228497541-d5b3237f-ecc5-472f-a015-f13d25ebf3d5.png">
