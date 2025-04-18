# How to Back Up Your iOS Device Locally (with Encrypted Backups)

Backing up your iOS device creates a copy of your data (your settings, photos, and app data). Encrypting your backups is **strongly recommended** to protect your data in case your computer gets into the wrong hands. Below you'll find instructions for creating an encrypted backup on macOS, Windows, and Linux.

> **Important:**  
> Choose an encryption password that is complex and not easily guessed. I recommend using a password manager to store this password safely. Losing your encryption password means you won't be able to restore your backup.


---

## **On macOS**

### **Using Finder (macOS Catalina 10.15 and later)**

1. **Connect Your iPhone:**
   - Plug your iPhone into your Mac using the USB cable.

2. **Open a Finder Window:**
   - Click the Finder icon in your Dock.
   - In the sidebar (left panel) under **Locations**, click on your iPhone.

3. **Trust Your Computer:**
   - On your iPhone, if prompted, tap **Trust This Computer** and enter your passcode.

4. **Back Up Your iPhone – Encrypting Your Backup:**
   - In the Finder window, click the **General** tab.
   - Select **"Back up all of the data on your iPhone to this Mac"**.
   - **IMPORTANT:** Check the **"Encrypt local backup"** box. This is strongly recommended to secure your data.
   - Enter a strong password and be sure to remember it.
   - Click **"Back Up Now"**.

5. **Locate Your Backup Files:**
   - Backups are stored in:  
     `~/Library/Application Support/MobileSync/Backup/`  
   - To view the folder, open Finder, press **Shift + Command + G**, and paste the path.

### **Using iTunes (macOS versions earlier than Catalina)**

1. **Connect Your iPhone:**
   - Use a USB cable to connect your iPhone to your Mac.

2. **Open iTunes:**
   - Launch iTunes.

3. **Select Your Device:**
   - Click on the iPhone icon near the top left side of the iTunes window.

4. **Encrypt and Back Up:**
   - In the **Summary** section, click **"Back Up Now"**.
   - **IMPORTANT:** Under the backup section, tick **"Encrypt local backup"**. Set a password when prompted and confirm it. This step is strongly recommended for protecting sensitive data.
   
5. **Find Your Backup Files:**
   - They are stored at:  
     `~/Library/Application Support/MobileSync/Backup/`

---

## **On Windows**

### **Using iTunes for Windows**

1. **Download and Install iTunes:**
   - If you haven’t installed it, download iTunes from Apple’s [official website](https://www.apple.com/itunes/download/).

2. **Connect Your iPhone:**
   - Plug your iPhone into your Windows computer with a USB cable.
   - On your iPhone, tap **Trust This Computer** if prompted.

3. **Open iTunes:**
   - Launch iTunes.

4. **Select Your Device:**
   - Click the iPhone icon in the top-left corner of the iTunes window.

5. **Encrypt and Back Up:**
   - In the **Summary** section, click **"Back Up Now"**.
   - **IMPORTANT:** Check the **"Encrypt local backup"** option. You will be asked to enter and verify a password. Using an encrypted backup is highly recommended.
   
6. **Locate Your Backup Files:**
   - Windows usually stores backups in:  
     `C:\Users\[YourUserName]\AppData\Roaming\Apple Computer\MobileSync\Backup\`  
   - Replace `[YourUserName]` with your actual username. You can paste this path into File Explorer’s address bar to access your backups.

---

## **On Linux**

Apple does not provide an official tool for backing up your iOS device on Linux. Instead, you can use an open-source tool called **libimobiledevice**. This guide uses `idevicebackup2` to create an encrypted backup.

### **Using libimobiledevice’s idevicebackup2**

1. **Install the Required Tools:**

   - **For Ubuntu/Debian-based Systems:**  
     Open Terminal and run:
     ```bash
     sudo apt-get update
     sudo apt-get install libimobiledevice6 libimobiledevice-utils ifuse
     ```
   
2. **Connect Your iPhone:**
   - Use the USB cable to connect your iPhone to your Linux computer.
   - On your iPhone, tap **Trust This Computer** if prompted.

3. **Pair Your Device:**
   - In the Terminal, run:
     ```bash
     idevicepair pair
     ```
   - You should see a confirmation that the pairing was successful.

4. **Create an Encrypted Backup:**
   - First, create a folder where your backup will be stored:
     ```bash
     mkdir ~/ios_backup
     ```
   - **IMPORTANT:** To create an encrypted backup (which is strongly recommended), run:
     ```bash
     idevicebackup2 backup --encrypted ~/ios_backup
     ```
   - You’ll be prompted to enter an encryption password. Use a strong password and remember it, because you'll need it to restore your backup.

5. **Verify the Backup:**
   - Once the command completes, your backup will be stored in the `~/ios_backup` folder.

6. **Additional Information:**
   - For further details or troubleshooting, visit the [libimobiledevice website](https://www.libimobiledevice.org/) or check its [GitHub repository](https://github.com/libimobiledevice/libimobiledevice).

