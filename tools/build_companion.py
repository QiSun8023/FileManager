"""Build the signed FileManager helper using the official Android SDK and a persistent private key.
Required: Android SDK platform android-36, build-tools 36.1.0, Java 17+.
FM_KEYSTORE and FM_KEYPASS_FILE must point outside the source repository. Never commit the private key.
"""
import argparse,os,subprocess,zipfile,shutil,hashlib
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
def run(args):
 subprocess.run([str(v) for v in args],check=True)
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--sdk',default=os.environ.get('ANDROID_SDK_ROOT',str(Path.home()/'AppData/Local/Android/Sdk')));a=parser.parse_args()
 sdk=Path(a.sdk);bt=sdk/'build-tools/36.1.0';android=sdk/'platforms/android-36/android.jar';keystore=Path(os.environ['FM_KEYSTORE']);pw=Path(os.environ['FM_KEYPASS_FILE'])
 if not keystore.is_file() or not pw.is_file():raise SystemExit('Missing persistent signing key/password file')
 if keystore.is_relative_to(ROOT) or pw.is_relative_to(ROOT):raise SystemExit('Keep signing credentials outside the source repository')
 build=ROOT/'companion/build';build.mkdir(exist_ok=True);classes=build/'classes';classes.mkdir(exist_ok=True);dex=build/'dex';dex.mkdir(exist_ok=True)
 run(['javac','-encoding','UTF-8','--release','17','-classpath',android,'-d',classes,*sorted((ROOT/'companion/src').rglob('*.java'))])
 jar=build/'classes.jar';run([Path(os.environ['JAVA_HOME'])/'bin/jar.exe','--create','--file',jar,'-C',classes,'.'])
 run([bt/'d8.bat','--release','--min-api','29','--lib',android,'--output',dex,jar])
 unsigned=build/'unsigned.apk';run([bt/'aapt2.exe','link','-I',android,'--manifest',ROOT/'companion/AndroidManifest.xml','-o',unsigned])
 with zipfile.ZipFile(unsigned,'a',compression=zipfile.ZIP_DEFLATED) as z:z.write(dex/'classes.dex','classes.dex')
 aligned=build/'aligned.apk';run([bt/'zipalign.exe','-f','4',unsigned,aligned])
 out=ROOT/'module/companion.apk';run([bt/'apksigner.bat','sign','--ks',keystore,'--ks-pass','file:'+str(pw),'--out',out,aligned]);run([bt/'apksigner.bat','verify','--verbose','--print-certs',out])
 (ROOT/'module/companion.sha256').write_text(hashlib.sha256(out.read_bytes()).hexdigest()+'\n',encoding='ascii',newline='\n');print('Built',out.name)
if __name__=='__main__':main()
